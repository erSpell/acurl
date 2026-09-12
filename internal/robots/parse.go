package robots

import (
	"bufio"
	"strings"
)

// group is one robots.txt record: the user agents it names and the
// non-standard lines acurl cares about.
type group struct {
	agents  []string
	signals map[string]string
}

// parsed holds what acurl extracts from robots.txt beyond allow/disallow,
// which grobotstxt already answers.
type parsed struct {
	groups   []group
	sitemaps []string
}

func parse(content string) parsed {
	var p parsed
	var cur *group
	lastWasAgent := false

	sc := bufio.NewScanner(strings.NewReader(content))
	sc.Buffer(make([]byte, 0, 64*1024), MaxSize)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)

		switch key {
		case "user-agent":
			if !lastWasAgent || cur == nil {
				p.groups = append(p.groups, group{signals: map[string]string{}})
				cur = &p.groups[len(p.groups)-1]
			}
			cur.agents = append(cur.agents, value)
			lastWasAgent = true
			continue
		case "sitemap":
			// Sitemap lines are global, whichever group they appear in.
			if value != "" {
				p.sitemaps = append(p.sitemaps, value)
			}
		case "content-signal":
			if cur != nil {
				for k, v := range parseSignals(value) {
					cur.signals[k] = v
				}
			}
		}
		lastWasAgent = false
	}
	return p
}

// parseSignals reads "search=yes, ai-train=no" into a map.
func parseSignals(v string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(v, ",") {
		k, val, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		k = strings.ToLower(strings.TrimSpace(k))
		val = strings.ToLower(strings.TrimSpace(val))
		if k != "" {
			out[k] = val
		}
	}
	return out
}

// groupFor returns the group that applies to agent: an exact
// (case-insensitive) product token match, else the * group.
func (p parsed) groupFor(agent string) *group {
	var star *group
	for i := range p.groups {
		for _, a := range p.groups[i].agents {
			if strings.EqualFold(a, agent) {
				return &p.groups[i]
			}
			if a == "*" && star == nil {
				star = &p.groups[i]
			}
		}
	}
	return star
}

// ContentSignals returns the Content-Signal directives
// (https://contentsignals.org) that apply to agent, such as
// {"search": "yes", "ai-train": "no"}. It returns nil when there are none.
func (d *Data) ContentSignals(agent string) map[string]string {
	g := parse(d.Content).groupFor(agent)
	if g == nil || len(g.signals) == 0 {
		return nil
	}
	return g.signals
}

// Sitemaps returns the sitemap URLs listed in robots.txt.
func (d *Data) Sitemaps() []string {
	return parse(d.Content).sitemaps
}
