package ui

import "strings"

const jsonPunctuation = "{}[],:"

func jsonStringEnd(line string, start int) int {
	for i := start + 1; i < len(line); i++ {
		switch line[i] {
		case '\\':
			i++
		case '"':
			return i + 1
		}
	}
	return len(line)
}

func colourJSON(line string) string {
	var out strings.Builder
	for i := 0; i < len(line); {
		c := line[i]
		switch {
		case c == ' ':
			out.WriteByte(c)
			i++
		case c == '"':
			end := jsonStringEnd(line, i)
			style := okStyle
			if strings.HasPrefix(strings.TrimLeft(line[end:], " "), ":") {
				style = accentStyle
			}
			out.WriteString(style.Render(line[i:end]))
			i = end
		case strings.IndexByte(jsonPunctuation, c) >= 0:
			out.WriteString(dimStyle.Render(string(c)))
			i++
		default:
			end := i
			for end < len(line) && !strings.ContainsRune(jsonPunctuation+" ", rune(line[end])) {
				end++
			}
			style := warnStyle
			if line[i:end] == "null" {
				style = dimStyle
			}
			out.WriteString(style.Render(line[i:end]))
			i = end
		}
	}
	return out.String()
}
