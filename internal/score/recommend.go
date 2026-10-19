package score

// recommendations explain how to fix each failed check.
var recommendations = map[string]string{
	"assistants-allowed": "AI assistants fetch pages when a shopper asks about them. Allow ChatGPT-User, " +
		"Claude-User and Perplexity-User in your WAF or bot-management rules (most vendors have a " +
		"\"verified AI bots\" category) and check that robots.txt does not disallow them.",
	"ai-search-allowed": "AI search indexes decide which products get cited. Allow OAI-SearchBot, " +
		"Claude-SearchBot and PerplexityBot in robots.txt and your firewall. You can still block " +
		"training crawlers such as GPTBot separately.",
	"no-challenges": "Agents cannot solve CAPTCHAs or JavaScript challenges. Skip challenges for " +
		"verified AI user agents, or allow-list their published IP ranges, instead of challenging every request.",
	"no-cloaking": "AI agents get a much smaller or different page than browsers. Serve the same " +
		"product content to everyone. Personalisation is fine, but stripping content for bots hurts how " +
		"assistants describe you. Run with --diff <profile> to see what is missing.",
	"server-rendered": "Most AI agents do not run JavaScript, so text rendered on the client is " +
		"invisible to them. Use server-side rendering or static generation for product pages, or at " +
		"least render the product name, price, availability and description in the initial HTML.",
	"structured-data": "Add a schema.org Product in JSON-LD with name, offers.price, " +
		"offers.priceCurrency and offers.availability. Agents read it to answer \"how much is it\" and " +
		"\"is it in stock\" without guessing from the page layout.",
	"indexable": "The page tells search engines not to index it (noindex), so search and AI search " +
		"indexes drop it and assistants cannot cite it. Remove noindex from the meta robots tag or " +
		"X-Robots-Tag on pages you want found, including any tag aimed at a specific crawler.",
	"product-parity": "Some AI agents see a different price or availability, or none at all. Make " +
		"sure caching, geo-pricing and bot rules give agents the same structured data a browser gets.",
}

func init() {
	for _, c := range checks {
		if recommendations[c.id] == "" {
			panic("score: missing recommendation for check " + c.id)
		}
	}
}
