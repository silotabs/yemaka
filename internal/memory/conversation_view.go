package memory

// WithInferredResponseParents returns a display copy of messages where older
// assistant replies without parent metadata are attached to the nearest user
// message. It does not mutate or repair stored rows; it only makes conversation
// reloads able to reconstruct response variants created by "try again".
func WithInferredResponseParents(messages []Message) []Message {
	if len(messages) == 0 {
		return nil
	}
	out := make([]Message, len(messages))
	copy(out, messages)

	preferPrevious := preferPreviousUserForDisplayOrder(out)
	for index := range out {
		if out[index].Role != "assistant" || out[index].ParentID != "" {
			continue
		}
		if parentID := inferNearestUserID(out, index, preferPrevious); parentID != "" {
			out[index].ParentID = parentID
		}
	}
	return out
}

func preferPreviousUserForDisplayOrder(messages []Message) bool {
	firstUser := -1
	firstAssistant := -1
	for index, message := range messages {
		switch message.Role {
		case "user":
			if firstUser == -1 {
				firstUser = index
			}
		case "assistant":
			if firstAssistant == -1 {
				firstAssistant = index
			}
		}
		if firstUser != -1 && firstAssistant != -1 {
			break
		}
	}
	return firstUser != -1 && (firstAssistant == -1 || firstUser < firstAssistant)
}

func inferNearestUserID(messages []Message, index int, preferPrevious bool) string {
	if preferPrevious {
		if id := scanUserID(messages, index-1, -1, -1); id != "" {
			return id
		}
		return scanUserID(messages, index+1, len(messages), 1)
	}
	if id := scanUserID(messages, index+1, len(messages), 1); id != "" {
		return id
	}
	return scanUserID(messages, index-1, -1, -1)
}

func scanUserID(messages []Message, start int, stop int, step int) string {
	for index := start; index != stop; index += step {
		if index < 0 || index >= len(messages) {
			return ""
		}
		if messages[index].Role == "user" && messages[index].ID != "" {
			return messages[index].ID
		}
	}
	return ""
}
