package linkdetect

import (
	"sort"
	"strings"

	"mvdan.cc/xurls/v2"
)

const (
	KindURL   = "URL"
	KindEmail = "Email"
)

type Link struct {
	Kind   string
	Label  string
	Target string
	Start  int
	End    int
}

type candidate struct {
	link  Link
	start int
}

const trimRight = ".,;:!?"

func Detect(value string) []Link {
	rx := xurls.Relaxed()
	emailIndex := rx.SubexpIndex("relaxedEmail")
	var candidates []candidate

	for _, match := range rx.FindAllStringSubmatchIndex(value, -1) {
		if len(match) < 2 || match[0] < 0 || match[1] < 0 {
			continue
		}
		raw := value[match[0]:match[1]]
		target := trimTarget(raw)
		if target == "" {
			continue
		}

		link := Link{
			Kind:   KindURL,
			Label:  target,
			Target: target,
		}
		if isRelaxedEmail(match, emailIndex) || strings.HasPrefix(strings.ToLower(target), "mailto:") {
			address := target
			if mailto := MailtoAddress(target); mailto != "" {
				address = mailto
			}
			link = Link{
				Kind:   KindEmail,
				Label:  address,
				Target: "mailto:" + address,
			}
		}

		candidates = append(candidates, candidate{link: link, start: match[0]})
		candidates[len(candidates)-1].link.Start = match[0]
		candidates[len(candidates)-1].link.End = match[0] + len(target)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].start < candidates[j].start
	})

	seen := make(map[string]bool)
	links := make([]Link, 0, len(candidates))
	for _, candidate := range candidates {
		key := strings.ToLower(candidate.link.Target)
		if seen[key] {
			continue
		}
		seen[key] = true
		links = append(links, candidate.link)
	}
	return links
}

func MailtoAddress(value string) string {
	if !strings.HasPrefix(strings.ToLower(value), "mailto:") {
		return ""
	}
	return strings.TrimSpace(value[len("mailto:"):])
}

func isRelaxedEmail(match []int, emailIndex int) bool {
	if emailIndex < 0 {
		return false
	}
	start := emailIndex * 2
	end := start + 1
	return end < len(match) && match[start] >= 0 && match[end] >= 0
}

func trimTarget(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "<>")
	value = strings.TrimRight(value, trimRight)
	return value
}
