package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"yemaka/internal/internet"
)

const maxCoreRequestsPerRun = 3

type ExtensionCapabilities struct {
	InternetFetch  bool     `json:"internetFetch"`
	InternetHead   bool     `json:"internetHead"`
	InternetSearch bool     `json:"internetSearch"`
	AllowedMethods []string `json:"allowedMethods,omitempty"`
	AllowedDomains []string `json:"allowedDomains,omitempty"`
}

type CoreRequest struct {
	ID    string         `json:"id"`
	Tool  string         `json:"tool"`
	Input map[string]any `json:"input,omitempty"`
}

type CoreResult struct {
	ID     string         `json:"id"`
	Tool   string         `json:"tool"`
	Status string         `json:"status"`
	Output map[string]any `json:"output,omitempty"`
	Error  string         `json:"error,omitempty"`
}

func extensionCapabilities(manifest Manifest) ExtensionCapabilities {
	capabilities := ExtensionCapabilities{
		AllowedMethods: append([]string{}, manifest.Permissions.Network.AllowedMethods...),
		AllowedDomains: append([]string{}, manifest.Permissions.Network.AllowedDomains...),
	}
	if manifest.Permissions.Network.Enabled {
		capabilities.InternetFetch = manifestAllowsMethod(manifest, http.MethodGet)
		capabilities.InternetHead = manifestAllowsMethod(manifest, http.MethodHead)
	}
	if permissionEnabled(manifest.Permissions.Extra, "internet_search") {
		capabilities.InternetSearch = true
	}
	return capabilities
}

func executeCoreRequests(ctx context.Context, manifest Manifest, options RunOptions, requests []CoreRequest) ([]CoreResult, error) {
	if len(requests) == 0 {
		return nil, nil
	}
	if len(requests) > maxCoreRequestsPerRun {
		return nil, fmt.Errorf("extension requested too many core calls: %d > %d", len(requests), maxCoreRequestsPerRun)
	}
	results := make([]CoreResult, 0, len(requests))
	for index, request := range requests {
		request.ID = strings.TrimSpace(request.ID)
		if request.ID == "" {
			request.ID = fmt.Sprintf("core_request_%d", index+1)
		}
		request.Tool = strings.TrimSpace(request.Tool)
		result := CoreResult{ID: request.ID, Tool: request.Tool, Status: "ok"}
		output, err := executeCoreRequest(ctx, manifest, options, request)
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
		} else {
			result.Output = output
		}
		results = append(results, result)
	}
	return results, nil
}

func executeCoreRequest(ctx context.Context, manifest Manifest, options RunOptions, request CoreRequest) (map[string]any, error) {
	switch request.Tool {
	case "internet_fetch":
		return executeInternetFetch(ctx, manifest, options, request, http.MethodGet)
	case "internet_head":
		return executeInternetFetch(ctx, manifest, options, request, http.MethodHead)
	case "internet_search":
		return executeInternetSearch(ctx, manifest, options, request)
	default:
		return nil, fmt.Errorf("unsupported core request tool: %s", request.Tool)
	}
}

func executeInternetFetch(ctx context.Context, manifest Manifest, options RunOptions, request CoreRequest, defaultMethod string) (map[string]any, error) {
	if options.Internet == nil {
		return nil, fmt.Errorf("core internet service is not available")
	}
	if !manifest.Permissions.Network.Enabled {
		return nil, fmt.Errorf("extension manifest does not grant network permission")
	}
	method := strings.ToUpper(strings.TrimSpace(stringFromAny(request.Input["method"])))
	if method == "" {
		method = defaultMethod
	}
	if request.Tool == "internet_head" {
		method = http.MethodHead
	}
	if !manifestAllowsMethod(manifest, method) {
		return nil, fmt.Errorf("network method %s is not allowed by extension manifest", method)
	}
	rawURL := stringFromAny(request.Input["url"])
	if rawURL == "" {
		rawURL = stringFromAny(request.Input["URL"])
	}
	if err := manifestAllowsURL(manifest, rawURL); err != nil {
		return nil, err
	}
	input := internet.FetchInput{
		URL:            rawURL,
		Method:         method,
		ExtractText:    boolFromAny(request.Input["extract_text"]) || boolFromAny(request.Input["extractText"]),
		AllowedDomains: append([]string{}, manifest.Permissions.Network.AllowedDomains...),
		TaskApproved:   manifest.Safety.RequiresUserApproval,
		Caller:         "extension:" + manifest.Name,
	}
	var result internet.FetchResult
	var err error
	if method == http.MethodHead {
		result, err = options.Internet.Head(ctx, input)
	} else {
		result, err = options.Internet.Fetch(ctx, input)
	}
	if err != nil {
		return nil, err
	}
	return structToMap(result)
}

func executeInternetSearch(ctx context.Context, manifest Manifest, options RunOptions, request CoreRequest) (map[string]any, error) {
	if options.Internet == nil {
		return nil, fmt.Errorf("core internet service is not available")
	}
	if !permissionEnabled(manifest.Permissions.Extra, "internet_search") {
		return nil, fmt.Errorf("extension manifest does not grant internet_search permission")
	}
	query := strings.TrimSpace(stringFromAny(request.Input["query"]))
	if query == "" {
		return nil, fmt.Errorf("internet_search query is required")
	}
	result, err := options.Internet.Search(ctx, internet.SearchInput{
		Query:        query,
		TaskApproved: manifest.Safety.RequiresUserApproval,
		MaxResults:   intFromAny(request.Input["max_results"]),
		Caller:       "extension:" + manifest.Name,
	})
	if err != nil {
		return nil, err
	}
	return structToMap(result)
}

func usesCoreNetworkBroker(manifest Manifest) bool {
	mode := strings.ToLower(strings.TrimSpace(manifest.Metadata["network_mode"]))
	return mode == "core_broker" || mode == "core-broker"
}

func manifestAllowsMethod(manifest Manifest, method string) bool {
	method = strings.ToUpper(strings.TrimSpace(method))
	for _, item := range manifest.Permissions.Network.AllowedMethods {
		if strings.ToUpper(strings.TrimSpace(item)) == method {
			return true
		}
	}
	return false
}

func manifestAllowsURL(manifest Manifest, rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return fmt.Errorf("url is required")
	}
	target, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	if target.Scheme == "" || target.Host == "" {
		return fmt.Errorf("url must include scheme and host")
	}
	host := strings.ToLower(strings.TrimSuffix(target.Hostname(), "."))
	for _, domain := range manifest.Permissions.Network.AllowedDomains {
		domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
		if domain == "" {
			continue
		}
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return nil
		}
	}
	return fmt.Errorf("domain %s is outside the extension manifest allowlist", host)
}

func permissionEnabled(extra map[string]Permission, key string) bool {
	if len(extra) == 0 {
		return false
	}
	permission, ok := extra[strings.ToLower(strings.TrimSpace(key))]
	return ok && permission.Enabled
}

func structToMap(value any) (map[string]any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var output map[string]any
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, err
	}
	return output, nil
}

func stringFromAny(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func boolFromAny(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}
