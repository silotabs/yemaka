package capabilities

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type Kind string

const (
	KindTool             Kind = "tool"
	KindSkill            Kind = "skill"
	KindDomainPack       Kind = "domain_pack"
	KindCapabilityPack   Kind = "capability_pack"
	KindExtension        Kind = "extension"
	KindInternetProvider Kind = "internet_provider"
	KindConnector        Kind = "connector"
	KindScheduler        Kind = "scheduler"
	KindCrawler          Kind = "crawler"
	KindNotification     Kind = "notification"
	KindModelRole        Kind = "model_role"
	KindLimitation       Kind = "limitation"
)

type State string

const (
	StateReady       State = "ready"
	StateDisabled    State = "disabled"
	StateNeedsConfig State = "needs_config"
	StateUnavailable State = "unavailable"
)

type GapAction string

const (
	ActionUseExisting       GapAction = "use_existing"
	ActionUsePack           GapAction = "use_pack"
	ActionGenerateExtension GapAction = "generate_extension"
	ActionAskConfig         GapAction = "ask_config"
	ActionAskClarification  GapAction = "ask_clarification"
	ActionUnsupported       GapAction = "unsupported"
)

type Capability struct {
	Name             string            `json:"name"`
	Kind             Kind              `json:"kind"`
	Description      string            `json:"description,omitempty"`
	State            State             `json:"state"`
	Available        bool              `json:"available"`
	Enabled          bool              `json:"enabled"`
	Configured       bool              `json:"configured"`
	RequiresApproval bool              `json:"requiresApproval,omitempty"`
	Source           string            `json:"source,omitempty"`
	Aliases          []string          `json:"aliases,omitempty"`
	Tags             []string          `json:"tags,omitempty"`
	Provides         []string          `json:"provides,omitempty"`
	Reason           string            `json:"reason,omitempty"`
	ConfigureHint    string            `json:"configureHint,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

type ModelRole struct {
	Role             string            `json:"role"`
	Model            string            `json:"model,omitempty"`
	Provider         string            `json:"provider,omitempty"`
	State            State             `json:"state"`
	Available        bool              `json:"available"`
	Enabled          bool              `json:"enabled"`
	Configured       bool              `json:"configured"`
	RequiresApproval bool              `json:"requiresApproval,omitempty"`
	Aliases          []string          `json:"aliases,omitempty"`
	Tags             []string          `json:"tags,omitempty"`
	Reason           string            `json:"reason,omitempty"`
	ConfigureHint    string            `json:"configureHint,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

type Limitation struct {
	Name            string   `json:"name"`
	Description     string   `json:"description,omitempty"`
	Reason          string   `json:"reason,omitempty"`
	SuggestedAction string   `json:"suggestedAction,omitempty"`
	Aliases         []string `json:"aliases,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	AppliesTo       []string `json:"appliesTo,omitempty"`
}

type Registry struct {
	Tools                []Capability `json:"tools,omitempty"`
	Skills               []Capability `json:"skills,omitempty"`
	DomainPacks          []Capability `json:"domainPacks,omitempty"`
	CapabilityPacks      []Capability `json:"capabilityPacks,omitempty"`
	Extensions           []Capability `json:"extensions,omitempty"`
	InternetProviders    []Capability `json:"internetProviders,omitempty"`
	Connectors           []Capability `json:"connectors,omitempty"`
	Notifications        []Capability `json:"notifications,omitempty"`
	Scheduler            Capability   `json:"scheduler,omitempty"`
	Crawler              Capability   `json:"crawler,omitempty"`
	ModelRoles           []ModelRole  `json:"modelRoles,omitempty"`
	KnownLimitations     []Limitation `json:"knownLimitations,omitempty"`
	DisabledCapabilities []Capability `json:"disabledCapabilities,omitempty"`
	ExtensionGeneration  Capability   `json:"extensionGeneration,omitempty"`
}

type GapQuery struct {
	Request     string `json:"request"`
	Capability  string `json:"capability,omitempty"`
	Kind        Kind   `json:"kind,omitempty"`
	Domain      string `json:"domain,omitempty"`
	AllowCreate bool   `json:"allowCreate,omitempty"`
}

type Match struct {
	Name   string `json:"name"`
	Kind   Kind   `json:"kind"`
	State  State  `json:"state"`
	Score  int    `json:"score"`
	Source string `json:"source,omitempty"`
}

type GapDecision struct {
	Action           GapAction   `json:"action"`
	Name             string      `json:"name,omitempty"`
	Kind             Kind        `json:"kind,omitempty"`
	State            State       `json:"state,omitempty"`
	Reason           string      `json:"reason"`
	SuggestedAction  string      `json:"suggestedAction,omitempty"`
	RequiresApproval bool        `json:"requiresApproval,omitempty"`
	Matches          []Match     `json:"matches,omitempty"`
	Missing          []string    `json:"missing,omitempty"`
	Capability       *Capability `json:"capability,omitempty"`
}

var nonTokenPattern = regexp.MustCompile(`[^a-z0-9]+`)

func Ready(name string, kind Kind) Capability {
	return Capability{
		Name:       name,
		Kind:       kind,
		State:      StateReady,
		Available:  true,
		Enabled:    true,
		Configured: true,
	}
}

func Disabled(name string, kind Kind, reason string) Capability {
	return Capability{
		Name:       name,
		Kind:       kind,
		State:      StateDisabled,
		Available:  true,
		Enabled:    false,
		Configured: true,
		Reason:     strings.TrimSpace(reason),
	}
}

func NeedsConfig(name string, kind Kind, reason string) Capability {
	return Capability{
		Name:       name,
		Kind:       kind,
		State:      StateNeedsConfig,
		Available:  true,
		Enabled:    false,
		Configured: false,
		Reason:     strings.TrimSpace(reason),
	}
}

func (r *Registry) AddTool(capability Capability) error {
	capability.Kind = KindTool
	return r.add(capability)
}

func (r *Registry) AddSkill(capability Capability) error {
	capability.Kind = KindSkill
	return r.add(capability)
}

func (r *Registry) AddDomainPack(capability Capability) error {
	capability.Kind = KindDomainPack
	return r.add(capability)
}

func (r *Registry) AddCapabilityPack(capability Capability) error {
	capability.Kind = KindCapabilityPack
	return r.add(capability)
}

func (r *Registry) AddExtension(capability Capability) error {
	capability.Kind = KindExtension
	return r.add(capability)
}

func (r *Registry) AddInternetProvider(capability Capability) error {
	capability.Kind = KindInternetProvider
	return r.add(capability)
}

func (r *Registry) AddConnector(capability Capability) error {
	capability.Kind = KindConnector
	return r.add(capability)
}

func (r *Registry) SetScheduler(capability Capability) error {
	capability.Kind = KindScheduler
	if strings.TrimSpace(capability.Name) == "" {
		capability.Name = "scheduler"
	}
	normalized, err := normalizeCapability(capability)
	if err != nil {
		return err
	}
	r.Scheduler = normalized
	return nil
}

func (r *Registry) SetCrawler(capability Capability) error {
	capability.Kind = KindCrawler
	if strings.TrimSpace(capability.Name) == "" {
		capability.Name = "crawler"
	}
	normalized, err := normalizeCapability(capability)
	if err != nil {
		return err
	}
	r.Crawler = normalized
	return nil
}

func (r *Registry) AddModelRole(role ModelRole) error {
	capability, err := normalizeCapability(role.capability())
	if err != nil {
		return err
	}
	role = modelRoleFromCapability(role, capability)
	if hasModelRole(r.ModelRoles, role.Role) {
		return fmt.Errorf("model role already registered: %s", role.Role)
	}
	r.ModelRoles = append(r.ModelRoles, role)
	sortModelRoles(r.ModelRoles)
	return nil
}

func (r *Registry) AddKnownLimitation(limitation Limitation) error {
	limitation = normalizeLimitation(limitation)
	if limitation.Name == "" {
		return fmt.Errorf("limitation name is required")
	}
	if hasLimitation(r.KnownLimitations, limitation.Name) {
		return fmt.Errorf("limitation already registered: %s", limitation.Name)
	}
	r.KnownLimitations = append(r.KnownLimitations, limitation)
	sortLimitations(r.KnownLimitations)
	return nil
}

func (r *Registry) AddDisabledCapability(capability Capability) error {
	if capability.State == "" {
		capability.State = StateDisabled
	}
	if capability.Kind == "" {
		return fmt.Errorf("disabled capability kind is required")
	}
	normalized, err := normalizeCapability(capability)
	if err != nil {
		return err
	}
	if normalized.State == StateReady {
		normalized.State = StateDisabled
		normalized.Enabled = false
	}
	if hasCapability(r.DisabledCapabilities, normalized.Kind, normalized.Name) {
		return fmt.Errorf("disabled capability already registered: %s", normalized.Name)
	}
	r.DisabledCapabilities = append(r.DisabledCapabilities, normalized)
	sortCapabilities(r.DisabledCapabilities)
	return nil
}

func (r *Registry) SetExtensionGeneration(capability Capability) error {
	capability.Kind = KindExtension
	if strings.TrimSpace(capability.Name) == "" {
		capability.Name = "extension_generation"
	}
	normalized, err := normalizeCapability(capability)
	if err != nil {
		return err
	}
	r.ExtensionGeneration = normalized
	return nil
}

func (r *Registry) add(capability Capability) error {
	normalized, err := normalizeCapability(capability)
	if err != nil {
		return err
	}
	list := r.listForKind(normalized.Kind)
	if hasCapability(*list, normalized.Kind, normalized.Name) {
		return fmt.Errorf("%s already registered: %s", normalized.Kind, normalized.Name)
	}
	*list = append(*list, normalized)
	sortCapabilities(*list)
	return nil
}

func (r *Registry) listForKind(kind Kind) *[]Capability {
	switch kind {
	case KindTool:
		return &r.Tools
	case KindSkill:
		return &r.Skills
	case KindDomainPack:
		return &r.DomainPacks
	case KindCapabilityPack:
		return &r.CapabilityPacks
	case KindExtension:
		return &r.Extensions
	case KindInternetProvider:
		return &r.InternetProviders
	case KindConnector:
		return &r.Connectors
	case KindNotification:
		return &r.Notifications
	default:
		return &r.Tools
	}
}

func (r Registry) Snapshot() Registry {
	out := Registry{}
	copyCaps := func(input []Capability) []Capability {
		items := make([]Capability, 0, len(input))
		for _, item := range input {
			normalized, err := normalizeCapability(item)
			if err == nil {
				items = append(items, normalized)
			}
		}
		sortCapabilities(items)
		return items
	}
	out.Tools = copyCaps(r.Tools)
	out.Skills = copyCaps(r.Skills)
	out.DomainPacks = copyCaps(r.DomainPacks)
	out.CapabilityPacks = copyCaps(r.CapabilityPacks)
	out.Extensions = copyCaps(r.Extensions)
	out.InternetProviders = copyCaps(r.InternetProviders)
	out.Connectors = copyCaps(r.Connectors)
	out.Notifications = copyCaps(r.Notifications)
	out.DisabledCapabilities = copyCaps(r.DisabledCapabilities)
	if strings.TrimSpace(r.Scheduler.Name) != "" || r.Scheduler.Kind != "" {
		out.Scheduler, _ = normalizeCapability(defaultNamedCapability(r.Scheduler, "scheduler", KindScheduler))
	}
	if strings.TrimSpace(r.Crawler.Name) != "" || r.Crawler.Kind != "" {
		out.Crawler, _ = normalizeCapability(defaultNamedCapability(r.Crawler, "crawler", KindCrawler))
	}
	if strings.TrimSpace(r.ExtensionGeneration.Name) != "" || r.ExtensionGeneration.Kind != "" {
		out.ExtensionGeneration, _ = normalizeCapability(defaultNamedCapability(r.ExtensionGeneration, "extension_generation", KindExtension))
	}
	for _, role := range r.ModelRoles {
		capability, err := normalizeCapability(role.capability())
		if err == nil {
			out.ModelRoles = append(out.ModelRoles, modelRoleFromCapability(role, capability))
		}
	}
	sortModelRoles(out.ModelRoles)
	for _, limitation := range r.KnownLimitations {
		normalized := normalizeLimitation(limitation)
		if normalized.Name != "" {
			out.KnownLimitations = append(out.KnownLimitations, normalized)
		}
	}
	sortLimitations(out.KnownLimitations)
	return out
}

func (r Registry) Get(kind Kind, name string) (Capability, bool) {
	name = normalizeKey(name)
	if name == "" {
		return Capability{}, false
	}
	for _, capability := range r.Snapshot().capabilities() {
		if capability.Kind == kind && normalizeKey(capability.Name) == name {
			return capability, true
		}
	}
	return Capability{}, false
}

func (r Registry) ClassifyGap(query GapQuery) GapDecision {
	snapshot := r.Snapshot()
	request := strings.TrimSpace(strings.Join([]string{query.Request, query.Capability, query.Domain}, " "))
	if request == "" {
		return GapDecision{
			Action:          ActionAskClarification,
			Reason:          "capability request is empty",
			SuggestedAction: "Ask what task or capability Yemaka should evaluate.",
		}
	}

	if limitation, score := snapshot.bestLimitationMatch(query, request); score > 0 {
		return GapDecision{
			Action:          ActionUnsupported,
			Name:            limitation.Name,
			Kind:            KindLimitation,
			Reason:          nonEmpty(limitation.Reason, limitation.Description, "this capability is recorded as unsupported"),
			SuggestedAction: nonEmpty(limitation.SuggestedAction, "Use a local skill, pack, connector, or generated extension only if policy allows it."),
			Matches:         []Match{{Name: limitation.Name, Kind: KindLimitation, State: StateUnavailable, Score: score}},
			Missing:         []string{limitation.Name},
		}
	}

	matches := snapshot.findMatches(query, request)
	if len(matches) > 0 {
		best := matches[0]
		if best.capability.State == StateReady {
			action := ActionUseExisting
			if isPack(best.capability.Kind) {
				action = ActionUsePack
			}
			return GapDecision{
				Action:           action,
				Name:             best.capability.Name,
				Kind:             best.capability.Kind,
				State:            best.capability.State,
				Reason:           readyReason(best.capability),
				SuggestedAction:  readySuggestedAction(best.capability),
				RequiresApproval: best.capability.RequiresApproval,
				Matches:          matchesForDecision(matches),
				Capability:       capabilityPtr(best.capability),
			}
		}
		if best.capability.State == StateDisabled || best.capability.State == StateNeedsConfig {
			return GapDecision{
				Action:           ActionAskConfig,
				Name:             best.capability.Name,
				Kind:             best.capability.Kind,
				State:            best.capability.State,
				Reason:           nonEmpty(best.capability.Reason, disabledReason(best.capability)),
				SuggestedAction:  configSuggestion(best.capability),
				RequiresApproval: best.capability.RequiresApproval,
				Matches:          matchesForDecision(matches),
				Missing:          []string{best.capability.Name},
				Capability:       capabilityPtr(best.capability),
			}
		}
		return GapDecision{
			Action:          ActionUnsupported,
			Name:            best.capability.Name,
			Kind:            best.capability.Kind,
			State:           best.capability.State,
			Reason:          nonEmpty(best.capability.Reason, "the matched capability is unavailable"),
			SuggestedAction: nonEmpty(best.capability.ConfigureHint, "Install or enable a pack/extension/connector outside the trusted core, then retry."),
			Matches:         matchesForDecision(matches),
			Missing:         []string{best.capability.Name},
			Capability:      capabilityPtr(best.capability),
		}
	}

	if looksConfigBacked(request, query.Kind) {
		return GapDecision{
			Action:          ActionAskConfig,
			Name:            configBackedName(request, query.Kind),
			Kind:            query.Kind,
			State:           StateNeedsConfig,
			Reason:          "the request needs an optional capability that is not ready in the capability registry",
			SuggestedAction: "Configure and explicitly enable the matching provider, connector, scheduler, crawler, model role, or pack before use.",
			Missing:         []string{configBackedName(request, query.Kind)},
		}
	}

	if looksActionable(request) || query.AllowCreate {
		generation := snapshot.ExtensionGeneration
		if generation.State == StateReady {
			return GapDecision{
				Action:           ActionGenerateExtension,
				Name:             "generated_extension",
				Kind:             KindExtension,
				State:            StateReady,
				Reason:           "no ready tool, skill, pack, connector, or extension matches this task",
				SuggestedAction:  "Propose the smallest profile-local generated extension, validate its manifest, run tests, and require approval before registration or use.",
				RequiresApproval: true,
				Missing:          []string{strings.TrimSpace(query.Capability)},
				Capability:       capabilityPtr(generation),
			}
		}
		if generation.State == StateDisabled || generation.State == StateNeedsConfig {
			return GapDecision{
				Action:          ActionAskConfig,
				Name:            generation.Name,
				Kind:            KindExtension,
				State:           generation.State,
				Reason:          nonEmpty(generation.Reason, "extension generation is not ready"),
				SuggestedAction: configSuggestion(generation),
				Missing:         []string{strings.TrimSpace(query.Capability)},
				Capability:      capabilityPtr(generation),
			}
		}
		return GapDecision{
			Action:          ActionUnsupported,
			Name:            "generated_extension",
			Kind:            KindExtension,
			State:           StateUnavailable,
			Reason:          "no existing capability matches and extension generation is not available in this registry",
			SuggestedAction: "Install or enable a suitable pack/extension through the approved capability flow.",
			Missing:         []string{strings.TrimSpace(query.Capability)},
		}
	}

	return GapDecision{
		Action:          ActionAskClarification,
		Reason:          "no deterministic capability match was found",
		SuggestedAction: "Ask for the concrete tool, skill, pack, connector, provider, model role, or workflow needed.",
		Missing:         []string{strings.TrimSpace(query.Capability)},
	}
}

type scoredCapability struct {
	capability Capability
	score      int
}

func (r Registry) capabilities() []Capability {
	items := make([]Capability, 0, len(r.Tools)+len(r.Skills)+len(r.DomainPacks)+len(r.CapabilityPacks)+len(r.Extensions)+len(r.InternetProviders)+len(r.Connectors)+len(r.Notifications)+len(r.DisabledCapabilities)+len(r.ModelRoles)+3)
	items = append(items, r.Tools...)
	items = append(items, r.Skills...)
	items = append(items, r.DomainPacks...)
	items = append(items, r.CapabilityPacks...)
	items = append(items, r.Extensions...)
	items = append(items, r.InternetProviders...)
	items = append(items, r.Connectors...)
	items = append(items, r.Notifications...)
	items = append(items, r.DisabledCapabilities...)
	if strings.TrimSpace(r.Scheduler.Name) != "" {
		items = append(items, r.Scheduler)
	}
	if strings.TrimSpace(r.Crawler.Name) != "" {
		items = append(items, r.Crawler)
	}
	if strings.TrimSpace(r.ExtensionGeneration.Name) != "" {
		items = append(items, r.ExtensionGeneration)
	}
	for _, role := range r.ModelRoles {
		items = append(items, role.capability())
	}
	return items
}

func (r Registry) findMatches(query GapQuery, request string) []scoredCapability {
	var scored []scoredCapability
	for _, capability := range r.capabilities() {
		if query.Kind != "" && capability.Kind != query.Kind && !(query.Kind == KindExtension && capability.Name == "extension_generation") {
			continue
		}
		score := scoreCapability(capability, query, request)
		if score > 0 {
			scored = append(scored, scoredCapability{capability: capability, score: score})
		}
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		if stateRank(scored[i].capability.State) != stateRank(scored[j].capability.State) {
			return stateRank(scored[i].capability.State) < stateRank(scored[j].capability.State)
		}
		if kindRank(scored[i].capability.Kind) != kindRank(scored[j].capability.Kind) {
			return kindRank(scored[i].capability.Kind) < kindRank(scored[j].capability.Kind)
		}
		return scored[i].capability.Name < scored[j].capability.Name
	})
	if len(scored) > 6 {
		scored = scored[:6]
	}
	return scored
}

func (r Registry) bestLimitationMatch(query GapQuery, request string) (Limitation, int) {
	var best Limitation
	bestScore := 0
	for _, limitation := range r.KnownLimitations {
		score := scoreLimitation(limitation, query, request)
		if score > bestScore || (score == bestScore && score > 0 && limitation.Name < best.Name) {
			best = limitation
			bestScore = score
		}
	}
	return best, bestScore
}

func scoreCapability(capability Capability, query GapQuery, request string) int {
	requestKey := normalizeText(request)
	requestTokens := tokenSet(request)
	explicit := normalizeKey(query.Capability)
	score := 0
	keys := capabilityKeys(capability)
	for _, key := range keys {
		if key == "" {
			continue
		}
		if explicit != "" && key == explicit {
			score += 120
		}
		if requestKey == key {
			score += 90
		}
		if strings.Contains(requestKey, key) {
			score += 45
		}
		if overlap := meaningfulKeyTokenOverlap(key, requestTokens); overlap > 0 {
			score += overlap
		}
	}
	for _, tag := range capability.Tags {
		if requestTokens[normalizeKey(tag)] {
			score += 8
		}
	}
	for _, provide := range capability.Provides {
		key := normalizeKey(provide)
		if key != "" && requestTokens[key] {
			score += 12
		}
	}
	if query.Domain != "" {
		domain := normalizeKey(query.Domain)
		for _, tag := range append(capability.Tags, capability.Source) {
			if normalizeKey(tag) == domain {
				score += 15
			}
		}
	}
	if score > 0 && query.Kind != "" && capability.Kind == query.Kind {
		score += 10
	}
	return score
}

func meaningfulKeyTokenOverlap(key string, requestTokens map[string]bool) int {
	tokens := meaningfulCapabilityKeyTokens(key)
	if len(tokens) < 2 {
		return 0
	}
	matches := 0
	for _, token := range tokens {
		if requestTokens[token] {
			matches++
		}
	}
	if matches < 2 {
		return 0
	}
	if len(tokens) <= 3 && matches < len(tokens)-1 {
		return 0
	}
	if len(tokens) > 3 && matches*2 < len(tokens) {
		return 0
	}
	return 18 + matches*4
}

func meaningfulCapabilityKeyTokens(key string) []string {
	out := []string{}
	for token := range tokenSet(key) {
		if isCapabilityMatchStopword(token) {
			continue
		}
		out = append(out, token)
	}
	sort.Strings(out)
	return out
}

func isCapabilityMatchStopword(token string) bool {
	switch token {
	case "a", "an", "and", "are", "as", "for", "from", "in", "into", "is", "it", "my", "of", "on", "or", "the", "these", "this", "to", "with":
		return true
	default:
		return false
	}
}

func scoreLimitation(limitation Limitation, query GapQuery, request string) int {
	requestKey := normalizeText(request)
	requestTokens := tokenSet(request)
	explicit := normalizeKey(query.Capability)
	score := 0
	for _, key := range limitationKeys(limitation) {
		if explicit != "" && key == explicit {
			score += 120
		}
		if requestKey == key {
			score += 90
		}
		if strings.Contains(requestKey, key) {
			score += 55
		}
	}
	for _, tag := range limitation.Tags {
		if requestTokens[normalizeKey(tag)] {
			score += 8
		}
	}
	if query.Kind != "" {
		for _, appliesTo := range limitation.AppliesTo {
			if normalizeKey(appliesTo) == normalizeKey(string(query.Kind)) {
				score += 12
			}
		}
	}
	return score
}

func capabilityKeys(capability Capability) []string {
	keys := []string{normalizeKey(capability.Name)}
	for _, value := range capability.Aliases {
		keys = append(keys, normalizeKey(value))
	}
	for _, value := range capability.Provides {
		keys = append(keys, normalizeKey(value))
	}
	return uniqueStrings(keys)
}

func limitationKeys(limitation Limitation) []string {
	keys := []string{normalizeKey(limitation.Name)}
	for _, value := range limitation.Aliases {
		keys = append(keys, normalizeKey(value))
	}
	for _, value := range limitation.AppliesTo {
		keys = append(keys, normalizeKey(value))
	}
	return uniqueStrings(keys)
}

func normalizeCapability(capability Capability) (Capability, error) {
	capability.Name = strings.TrimSpace(capability.Name)
	capability.Description = strings.TrimSpace(capability.Description)
	capability.Source = strings.TrimSpace(capability.Source)
	capability.Reason = strings.TrimSpace(capability.Reason)
	capability.ConfigureHint = strings.TrimSpace(capability.ConfigureHint)
	capability.Kind = Kind(strings.TrimSpace(strings.ToLower(string(capability.Kind))))
	if capability.Name == "" {
		return Capability{}, fmt.Errorf("capability name is required")
	}
	if !validKind(capability.Kind) {
		return Capability{}, fmt.Errorf("unsupported capability kind: %s", capability.Kind)
	}
	capability.State = normalizeState(capability)
	switch capability.State {
	case StateReady:
		capability.Available = true
		capability.Enabled = true
		capability.Configured = true
	case StateDisabled:
		capability.Available = true
		capability.Enabled = false
	case StateNeedsConfig:
		capability.Available = true
		capability.Configured = false
	case StateUnavailable:
		capability.Available = false
		capability.Enabled = false
		capability.Configured = false
	}
	capability.Aliases = normalizeList(capability.Aliases)
	capability.Tags = normalizeList(capability.Tags)
	capability.Provides = normalizeList(capability.Provides)
	capability.Metadata = copyMetadata(capability.Metadata)
	return capability, nil
}

func normalizeState(capability Capability) State {
	state := State(strings.TrimSpace(strings.ToLower(string(capability.State))))
	switch state {
	case StateReady, StateDisabled, StateNeedsConfig, StateUnavailable:
		return state
	}
	switch {
	case capability.Available && capability.Enabled && capability.Configured:
		return StateReady
	case capability.Available && capability.Enabled && !capability.Configured:
		return StateNeedsConfig
	case capability.Available && !capability.Enabled:
		return StateDisabled
	default:
		return StateUnavailable
	}
}

func normalizeLimitation(limitation Limitation) Limitation {
	limitation.Name = strings.TrimSpace(limitation.Name)
	limitation.Description = strings.TrimSpace(limitation.Description)
	limitation.Reason = strings.TrimSpace(limitation.Reason)
	limitation.SuggestedAction = strings.TrimSpace(limitation.SuggestedAction)
	limitation.Aliases = normalizeList(limitation.Aliases)
	limitation.Tags = normalizeList(limitation.Tags)
	limitation.AppliesTo = normalizeList(limitation.AppliesTo)
	return limitation
}

func validKind(kind Kind) bool {
	switch kind {
	case KindTool, KindSkill, KindDomainPack, KindCapabilityPack, KindExtension, KindInternetProvider, KindConnector, KindNotification, KindScheduler, KindCrawler, KindModelRole:
		return true
	default:
		return false
	}
}

func normalizeList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := normalizeKey(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool {
		return normalizeKey(out[i]) < normalizeKey(out[j])
	})
	return out
}

func normalizeKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = nonTokenPattern.ReplaceAllString(value, "_")
	return strings.Trim(value, "_")
}

func normalizeText(value string) string {
	return normalizeKey(value)
}

func tokenSet(value string) map[string]bool {
	out := map[string]bool{}
	for _, token := range strings.Split(normalizeText(value), "_") {
		if token != "" {
			out[token] = true
		}
	}
	return out
}

func uniqueStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func sortCapabilities(items []Capability) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return kindRank(items[i].Kind) < kindRank(items[j].Kind)
		}
		return normalizeKey(items[i].Name) < normalizeKey(items[j].Name)
	})
}

func sortModelRoles(items []ModelRole) {
	sort.Slice(items, func(i, j int) bool {
		return normalizeKey(items[i].Role) < normalizeKey(items[j].Role)
	})
}

func sortLimitations(items []Limitation) {
	sort.Slice(items, func(i, j int) bool {
		return normalizeKey(items[i].Name) < normalizeKey(items[j].Name)
	})
}

func hasCapability(items []Capability, kind Kind, name string) bool {
	key := normalizeKey(name)
	for _, item := range items {
		if item.Kind == kind && normalizeKey(item.Name) == key {
			return true
		}
	}
	return false
}

func hasModelRole(items []ModelRole, role string) bool {
	key := normalizeKey(role)
	for _, item := range items {
		if normalizeKey(item.Role) == key {
			return true
		}
	}
	return false
}

func hasLimitation(items []Limitation, name string) bool {
	key := normalizeKey(name)
	for _, item := range items {
		if normalizeKey(item.Name) == key {
			return true
		}
	}
	return false
}

func (r ModelRole) capability() Capability {
	name := strings.TrimSpace(r.Role)
	if name == "" {
		name = strings.TrimSpace(r.Model)
	}
	description := strings.TrimSpace(strings.Join([]string{r.Provider, r.Model}, " "))
	return Capability{
		Name:             name,
		Kind:             KindModelRole,
		Description:      description,
		State:            r.State,
		Available:        r.Available,
		Enabled:          r.Enabled,
		Configured:       r.Configured,
		RequiresApproval: r.RequiresApproval,
		Aliases:          r.Aliases,
		Tags:             r.Tags,
		Provides:         []string{r.Role, r.Model, r.Provider},
		Reason:           r.Reason,
		ConfigureHint:    r.ConfigureHint,
		Metadata:         r.Metadata,
	}
}

func modelRoleFromCapability(role ModelRole, capability Capability) ModelRole {
	role.Role = capability.Name
	role.State = capability.State
	role.Available = capability.Available
	role.Enabled = capability.Enabled
	role.Configured = capability.Configured
	role.RequiresApproval = capability.RequiresApproval
	role.Aliases = capability.Aliases
	role.Tags = capability.Tags
	role.Reason = capability.Reason
	role.ConfigureHint = capability.ConfigureHint
	return role
}

func defaultNamedCapability(capability Capability, name string, kind Kind) Capability {
	if strings.TrimSpace(capability.Name) == "" {
		capability.Name = name
	}
	if capability.Kind == "" {
		capability.Kind = kind
	}
	return capability
}

func isPack(kind Kind) bool {
	return kind == KindDomainPack || kind == KindCapabilityPack
}

func matchesForDecision(matches []scoredCapability) []Match {
	out := make([]Match, 0, len(matches))
	for _, match := range matches {
		out = append(out, Match{
			Name:   match.capability.Name,
			Kind:   match.capability.Kind,
			State:  match.capability.State,
			Score:  match.score,
			Source: match.capability.Source,
		})
	}
	return out
}

func capabilityPtr(capability Capability) *Capability {
	copy := capability
	return &copy
}

func readyReason(capability Capability) string {
	if isPack(capability.Kind) {
		return "an enabled pack declares this capability"
	}
	return "a ready existing capability matches this task"
}

func readySuggestedAction(capability Capability) string {
	if capability.RequiresApproval {
		return "Use the existing capability through the normal policy and approval flow."
	}
	if isPack(capability.Kind) {
		return "Use the enabled pack instead of generating new trusted-core code."
	}
	return "Use the existing capability."
}

func disabledReason(capability Capability) string {
	switch capability.State {
	case StateNeedsConfig:
		return "the matched capability needs configuration before use"
	case StateDisabled:
		return "the matched capability is disabled"
	default:
		return "the matched capability is not ready"
	}
}

func configSuggestion(capability Capability) string {
	if strings.TrimSpace(capability.ConfigureHint) != "" {
		return capability.ConfigureHint
	}
	switch capability.Kind {
	case KindInternetProvider:
		return "Configure an internet provider explicitly; internet and search remain disabled until the profile enables them."
	case KindConnector:
		return "Enable and configure only the connector needed, using secret references instead of secret values."
	case KindNotification:
		return "Enable notifications only for approved local workflows or scheduled jobs; keep remote push services disabled by default."
	case KindScheduler:
		return "Enable scheduler support and require approval for any new job."
	case KindCrawler:
		return "Configure crawler support only for crawl tasks; normal search/fetch should stay on the core internet service."
	case KindExtension:
		return "Enable generated extensions only through the manifest, test, approval, audit, and rollback flow."
	case KindDomainPack, KindCapabilityPack:
		return "Install and enable the pack explicitly before routing work to it."
	default:
		return "Enable or configure the capability explicitly before use."
	}
}

func looksConfigBacked(request string, kind Kind) bool {
	if kind == KindInternetProvider || kind == KindConnector || kind == KindNotification || kind == KindScheduler || kind == KindCrawler || kind == KindModelRole || kind == KindDomainPack || kind == KindCapabilityPack {
		return true
	}
	lower := normalizeText(request)
	groups := [][]string{
		{"internet", "web_search", "search_web", "provider", "searxng", "brave"},
		{"connector", "connect", "slack", "discord", "telegram", "email", "mcp"},
		{"notification", "notifications", "notify", "inbox"},
		{"scheduler", "schedule", "cron", "every_hour", "job"},
		{"crawler", "crawl", "crawler_backend", "spider"},
		{"model", "model_role", "coding_model", "reasoning_model"},
		{"domain_pack", "capability_pack", "pack"},
	}
	for _, group := range groups {
		for _, term := range group {
			if strings.Contains(lower, term) {
				return true
			}
		}
	}
	return false
}

func configBackedName(request string, kind Kind) string {
	if kind != "" {
		return string(kind)
	}
	lower := normalizeText(request)
	switch {
	case strings.Contains(lower, "crawler") || strings.Contains(lower, "crawl"):
		return "crawler"
	case strings.Contains(lower, "scheduler") || strings.Contains(lower, "schedule") || strings.Contains(lower, "cron"):
		return "scheduler"
	case strings.Contains(lower, "connector") || strings.Contains(lower, "slack") || strings.Contains(lower, "discord") || strings.Contains(lower, "telegram") || strings.Contains(lower, "email"):
		return "connector"
	case strings.Contains(lower, "notification") || strings.Contains(lower, "notify") || strings.Contains(lower, "inbox"):
		return "notification"
	case strings.Contains(lower, "model"):
		return "model_role"
	case strings.Contains(lower, "pack"):
		return "pack"
	default:
		return "internet_provider"
	}
}

func looksActionable(request string) bool {
	lower := normalizeText(request)
	terms := []string{
		"build", "create", "generate", "automate", "monitor", "convert", "extract", "normalize",
		"import", "export", "sync", "run", "schedule", "fetch", "crawl", "connect", "post", "publish",
	}
	for _, term := range terms {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

func stateRank(state State) int {
	switch state {
	case StateReady:
		return 0
	case StateNeedsConfig:
		return 1
	case StateDisabled:
		return 2
	default:
		return 3
	}
}

func kindRank(kind Kind) int {
	switch kind {
	case KindTool:
		return 0
	case KindSkill:
		return 1
	case KindDomainPack:
		return 2
	case KindCapabilityPack:
		return 3
	case KindExtension:
		return 4
	case KindInternetProvider:
		return 5
	case KindConnector:
		return 6
	case KindNotification:
		return 7
	case KindScheduler:
		return 8
	case KindCrawler:
		return 9
	case KindModelRole:
		return 10
	default:
		return 99
	}
}

func nonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
