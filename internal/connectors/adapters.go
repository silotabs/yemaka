package connectors

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type AdapterInput struct {
	Content string `json:"content"`
	Skill   string `json:"skill,omitempty"`
	User    string `json:"user,omitempty"`
	Channel string `json:"channel,omitempty"`
	Subject string `json:"subject,omitempty"`
}

func ParseAdapterInput(adapter string, contentType string, body []byte) (AdapterInput, error) {
	if !IsAdapter(adapter) {
		return AdapterInput{}, fmt.Errorf("unknown adapter: %s", adapter)
	}
	if len(body) == 0 {
		return AdapterInput{}, fmt.Errorf("request body is required")
	}
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	var input AdapterInput
	var err error
	if contentType == "application/x-www-form-urlencoded" {
		input, err = parseAdapterForm(adapter, string(body))
	} else {
		input, err = parseAdapterJSON(adapter, body)
	}
	if err != nil {
		return AdapterInput{}, err
	}
	input.Content = strings.TrimSpace(input.Content)
	input.Skill = strings.TrimSpace(input.Skill)
	input.User = strings.TrimSpace(input.User)
	input.Channel = strings.TrimSpace(input.Channel)
	input.Subject = strings.TrimSpace(input.Subject)
	if input.Content == "" {
		return AdapterInput{}, fmt.Errorf("adapter content is required")
	}
	return input, nil
}

func AdapterResponse(adapter string, result CoreResult) any {
	switch strings.TrimSpace(adapter) {
	case Slack:
		return map[string]any{
			"response_type": "ephemeral",
			"text":          result.Text,
			"model":         result.Model,
		}
	case Discord:
		return map[string]any{
			"content": result.Text,
			"model":   result.Model,
		}
	case Telegram:
		return map[string]any{
			"text":  result.Text,
			"model": result.Model,
		}
	case Email:
		return map[string]any{
			"subject": "Yemaka response",
			"body":    result.Text,
			"model":   result.Model,
		}
	default:
		return result
	}
}

func parseAdapterForm(adapter string, body string) (AdapterInput, error) {
	values, err := url.ParseQuery(body)
	if err != nil {
		return AdapterInput{}, fmt.Errorf("parse adapter form: %w", err)
	}
	input := AdapterInput{
		Content: firstValue(values, "content", "text", "body", "message"),
		Skill:   firstValue(values, "skill"),
		User:    firstValue(values, "user_name", "user_id", "from", "sender"),
		Channel: firstValue(values, "channel_name", "channel_id", "chat_id", "to"),
		Subject: firstValue(values, "subject"),
	}
	if adapter == Email {
		input.Content = joinEmailContent(input.Subject, input.Content)
	}
	return input, nil
}

func parseAdapterJSON(adapter string, body []byte) (AdapterInput, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return AdapterInput{}, fmt.Errorf("parse adapter JSON: %w", err)
	}
	input := AdapterInput{
		Content: firstString(payload, "content", "text", "body", "message"),
		Skill:   firstString(payload, "skill"),
		User:    firstString(payload, "user", "username", "from", "sender"),
		Channel: firstString(payload, "channel", "channel_id", "chat_id", "to"),
		Subject: firstString(payload, "subject"),
	}
	if input.Content == "" {
		if nested := objectValue(payload, "message"); nested != nil {
			input.Content = firstString(nested, "text", "caption", "content", "body")
			input.User = fallback(input.User, firstString(nested, "from", "sender", "username"))
			input.Channel = fallback(input.Channel, firstString(nested, "chat_id", "chat", "channel"))
		}
	}
	if input.Content == "" {
		if nested := objectValue(payload, "event"); nested != nil {
			input.Content = firstString(nested, "text", "content", "body")
			input.User = fallback(input.User, firstString(nested, "user", "username"))
			input.Channel = fallback(input.Channel, firstString(nested, "channel", "channel_id"))
		}
	}
	if input.Content == "" {
		if nested := objectValue(payload, "email"); nested != nil {
			input.Subject = fallback(input.Subject, firstString(nested, "subject"))
			input.Content = firstString(nested, "body", "text", "content")
			input.User = fallback(input.User, firstString(nested, "from", "sender"))
		}
	}
	if adapter == Email {
		input.Content = joinEmailContent(input.Subject, input.Content)
	}
	return input, nil
}

func firstValue(values url.Values, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(values.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

func firstString(values map[string]any, names ...string) string {
	for _, name := range names {
		value, ok := values[name]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			if text := strings.TrimSpace(typed); text != "" {
				return text
			}
		case float64:
			return fmt.Sprintf("%.0f", typed)
		}
	}
	return ""
}

func objectValue(values map[string]any, name string) map[string]any {
	value, ok := values[name]
	if !ok {
		return nil
	}
	typed, _ := value.(map[string]any)
	return typed
}

func fallback(current string, next string) string {
	if strings.TrimSpace(current) != "" {
		return current
	}
	return next
}

func joinEmailContent(subject string, body string) string {
	subject = strings.TrimSpace(subject)
	body = strings.TrimSpace(body)
	if subject == "" {
		return body
	}
	if body == "" {
		return subject
	}
	return "Subject: " + subject + "\n\n" + body
}
