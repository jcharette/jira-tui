package mentiondetect

import "regexp"

type Mention struct {
	Query string
	Start int
	End   int
}

var mentionPattern = regexp.MustCompile(`(^|[^\w@])@([A-Za-z][A-Za-z0-9._-]{1,60})`)

func Detect(value string) []Mention {
	matches := mentionPattern.FindAllStringSubmatchIndex(value, -1)
	mentions := make([]Mention, 0, len(matches))
	for _, match := range matches {
		if len(match) < 6 || match[4] < 0 || match[5] < 0 {
			continue
		}
		mentions = append(mentions, Mention{
			Query: value[match[4]:match[5]],
			Start: match[4] - 1,
			End:   match[5],
		})
	}
	return mentions
}
