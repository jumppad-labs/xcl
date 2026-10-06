package highlight

// stripJSONC returns src with the parts JSON does not allow but VS Code
// colour themes commonly use removed: // line comments, /* */ block comments
// and trailing commas before ] or }. Strings are left as they are, escapes
// included, so a // inside a string is not taken for a comment.
func stripJSONC(src []byte) []byte {
	withoutComments := make([]byte, 0, len(src))

	for offset := 0; offset < len(src); {
		c := src[offset]

		switch {
		case c == '"':
			end := stringEnd(src, offset)
			withoutComments = append(withoutComments, src[offset:end]...)
			offset = end
		case c == '/' && offset+1 < len(src) && src[offset+1] == '/':
			for offset < len(src) && src[offset] != '\n' {
				offset++
			}
		case c == '/' && offset+1 < len(src) && src[offset+1] == '*':
			offset += 2
			for offset < len(src) && !(src[offset] == '*' && offset+1 < len(src) && src[offset+1] == '/') {
				offset++
			}
			offset += 2
			// a comment separates the text around it, as whitespace would
			withoutComments = append(withoutComments, ' ')
		default:
			withoutComments = append(withoutComments, c)
			offset++
		}
	}

	return removeTrailingCommas(withoutComments)
}

// removeTrailingCommas drops each comma that only whitespace separates from
// a following ] or }, outside strings
func removeTrailingCommas(src []byte) []byte {
	out := make([]byte, 0, len(src))

	for offset := 0; offset < len(src); {
		c := src[offset]

		if c == '"' {
			end := stringEnd(src, offset)
			out = append(out, src[offset:end]...)
			offset = end
			continue
		}

		if c == ',' {
			next := offset + 1
			for next < len(src) && isSpace(src[next]) {
				next++
			}
			if next < len(src) && (src[next] == ']' || src[next] == '}') {
				offset++
				continue
			}
		}

		out = append(out, c)
		offset++
	}

	return out
}

// stringEnd returns the offset just past the JSON string starting at start,
// honouring backslash escapes, or the end of src when it is not closed
func stringEnd(src []byte, start int) int {
	for offset := start + 1; offset < len(src); offset++ {
		switch src[offset] {
		case '\\':
			offset++
		case '"':
			return offset + 1
		}
	}

	return len(src)
}
