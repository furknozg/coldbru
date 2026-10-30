package mock

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/furknozg/coldbru/internal/proposal/llm"
)

// Model is the model name the mock reports in its llm_call records.
const Model = "mock"

// DefaultDSL is the filter of a brief the mock has no hand-written answer for (D-072): everyone who
// ordered at least once.
const DefaultDSL = `totalOrders >= 1`

// Copy is the text of one case in one language. Line is one sentence of at most 100 characters; the
// SMS is "{{first_name}}, " + Line, the email builds its blocks around Line and Head.
type Copy struct {
	Line string
	Head string
}

// Case is one hand-written answer: a brief, the DSL filter for it, its title and its texts.
type Case struct {
	// Name labels the case in tests and logs.
	Name string
	// Brief is the brief the case answers; the match is on the normalized text.
	Brief string
	// Title is the campaign title Screen returns (D-088): Turkish, at most 60 characters.
	Title string
	// DSL is the filter ProposeSegment returns.
	DSL string
	TR  Copy
	EN  Copy
}

// SampleBrief is the example brief of the case study.
const SampleBrief = "Son 30 günde sepetine ürün ekleyip alışveriş yapmayan İstanbul'daki kullanıcılara %30 indirim hatırlatması gönder. Samimi ama kısa olsun."

// activeDSL stands for the account status "active" of the stub campaigns, which is not a filter
// field: a customer who was active in the last 90 days.
const activeDSL = `lastActiveDate >= ago("90d")`

// cases are the sample brief and the 12 past campaigns of stub/generate_stub_data.py (CAMPAIGNS),
// in that order. The DSL of a past campaign translates its segment summary field by field.
var cases = []Case{
	{
		Name: "sample", Brief: SampleBrief,
		Title: "İstanbul sepet hatırlatması %30",
		DSL:   `city === "İstanbul" && cartItems.some(c => c.addedAt >= ago("30d")) && orders.none(o => o.orderDate >= ago("30d"))`,
		TR:    Copy{Line: "Sepetindeki ürünler seni bekliyor: %30 indirim fırsatını kaçırma!", Head: "Sepetindeki ürünler için %30 indirim"},
		EN:    Copy{Line: "The items in your basket are waiting: do not miss 30% off!", Head: "30% off the items in your basket"},
	},
	{
		Name: "spring-refresh", Brief: "Home & Living'de yeni sezon ürünlerini tüm aktif kullanıcılara duyur.",
		Title: "Ev & Yaşam yeni sezon duyurusu",
		DSL:   activeDSL,
		TR:    Copy{Line: "Ev ve yaşam kategorisinde yeni sezon ürünleri geldi, hemen keşfet!", Head: "Yeni sezon ürünleri geldi"},
		EN:    Copy{Line: "New season home and living products have arrived, discover them now!", Head: "The new season has arrived"},
	},
	{
		Name: "welcome-may", Brief: "Son 30 günde kayıt olanlara %10 hoş geldin indirimi gönder.",
		Title: "Yeni üyelere %10 hoş geldin indirimi",
		DSL:   `signupDate >= ago("30d")`,
		TR:    Copy{Line: "Aramıza hoş geldin! İlk alışverişinde %10 hoş geldin indiriminden yararlan.", Head: "Hoş geldin indirimi: %10"},
		EN:    Copy{Line: "Welcome aboard! Enjoy 10% off with our welcome discount.", Head: "Welcome discount: 10% off"},
	},
	{
		Name: "cart-june", Brief: "Sepetinde ürün bırakanlara 24 saat içinde hatırlatma gönder.",
		Title: "Sepet hatırlatması",
		DSL:   `hasOpenCart === true`,
		TR:    Copy{Line: "Sepetinde ürünler var, alışverişini tamamlamak için hâlâ vaktin var.", Head: "Sepetin seni bekliyor"},
		EN:    Copy{Line: "You left items in your basket, there is still time to complete your order.", Head: "Your basket is waiting"},
	},
	{
		Name: "summer-kickoff", Brief: "Tüm kullanıcılara %30 yaz indirimi. Samimi ve kısa olsun.",
		Title: "%30 yaz indirimi",
		DSL:   activeDSL,
		TR:    Copy{Line: "Yaz indirimi başladı: %30 indirim seni bekliyor!", Head: "Yaz indirimi başladı"},
		EN:    Copy{Line: "The summer sale has started: 30% off is waiting for you!", Head: "The summer sale has started"},
	},
	{
		Name: "summer-last-call", Brief: "Yaz indiriminden yararlanmayan kullanıcılara son çağrı.",
		Title: "Yaz indirimi son çağrı",
		DSL:   activeDSL + ` && orders.none(o => o.orderDate >= ago("30d"))`,
		TR:    Copy{Line: "Yaz indirimi sona eriyor, son çağrı: fırsatı kaçırma!", Head: "Yaz indirimi için son çağrı"},
		EN:    Copy{Line: "The summer sale is ending, last call: do not miss out!", Head: "Last call for the summer sale"},
	},
	{
		Name: "back-to-school", Brief: "Çocuklu kullanıcılara okul dönüşü ürünleri.",
		Title: "Okul dönüşü ürünleri",
		DSL:   `dependants >= 1`,
		TR:    Copy{Line: "Okul dönüşü için çocuğuna özel ürünleri keşfet.", Head: "Okul dönüşü ürünleri"},
		EN:    Copy{Line: "Discover products picked for the back-to-school season.", Head: "Back to school"},
	},
	{
		Name: "istanbul-flash", Brief: "İstanbul'daki kullanıcılara 48 saatlik flash indirim.",
		Title: "İstanbul 48 saatlik flash indirim",
		DSL:   `city === "İstanbul"`,
		TR:    Copy{Line: "İstanbul'a özel 48 saatlik flash indirim başladı, acele et!", Head: "48 saatlik flash indirim"},
		EN:    Copy{Line: "A 48-hour flash deal for Istanbul has started, hurry up!", Head: "48-hour flash deal"},
	},
	{
		Name: "loyalty-gold", Brief: "Gold ve Platinum üyelere özel erken erişim.",
		Title: "Gold ve Platinum erken erişim",
		DSL:   `loyaltyTier in ["Gold", "Platinum"]`,
		TR:    Copy{Line: "Gold ve Platinum üyelere özel erken erişim fırsatı seni bekliyor.", Head: "Üyelere özel erken erişim"},
		EN:    Copy{Line: "Early access for Gold and Platinum members is waiting for you.", Head: "Early access for members"},
	},
	{
		Name: "cart-september", Brief: "Son 7 günde sepete ekleyip almayanlara %15 hatırlatma.",
		Title: "Sepet hatırlatması %15",
		DSL:   `lastCartAddDate >= ago("7d") && orders.none(o => o.orderDate >= ago("7d"))`,
		TR:    Copy{Line: "Sepetine eklediğin ürünler için %15 indirim seni bekliyor!", Head: "Sepetin için %15 indirim"},
		EN:    Copy{Line: "15% off the items you added to your basket is waiting for you!", Head: "15% off your basket"},
	},
	{
		Name: "autumn-fashion", Brief: "Moda kategorisini seven kullanıcılara yeni koleksiyon.",
		Title: "Moda yeni koleksiyon",
		DSL:   `favouriteCategory === "Fashion"`,
		TR:    Copy{Line: "Yeni moda koleksiyonu geldi, favori parçalarını keşfet.", Head: "Yeni koleksiyon geldi"},
		EN:    Copy{Line: "The new fashion collection is here, discover your favourite pieces.", Head: "The new collection is here"},
	},
	{
		Name: "win-back-60d", Brief: "60 gündür alışveriş yapmayan eski müşterilere geri dönüş teklifi.",
		Title: "60 gün geri kazanım teklifi",
		DSL:   `lastOrderDate <= ago("60d") && totalOrders >= 1`,
		TR:    Copy{Line: "Seni özledik! Dönmen için hazırladığımız teklifi görmek ister misin?", Head: "Seni özledik"},
		EN:    Copy{Line: "We miss you! Would you like to see the offer we made for your return?", Head: "We miss you"},
	},
	{
		Name: "republic-day", Brief: "29 Ekim haftasonu için tüm TR kullanıcılarına %20 indirim.",
		Title: "29 Ekim haftasonu %20 indirim",
		DSL:   `countryCode === "TR"`,
		TR:    Copy{Line: "29 Ekim haftasonuna özel %20 indirim seni bekliyor!", Head: "Cumhuriyet haftasonu %20 indirim"},
		EN:    Copy{Line: "A 20% off deal for the 29 October weekend is waiting for you!", Head: "Republic Day weekend: 20% off"},
	},
}

// unmatched is the text of a brief without a hand-written answer: it is marked, so that nobody
// mistakes it for a model's work (D-072).
var unmatched = Case{
	Name: "default", DSL: DefaultDSL, Title: "[mock] Örnek kampanya",
	TR: Copy{Line: "[mock] Bu örnek bir yanıttır; gerçek model bağlanınca metin brief'e göre yazılır.", Head: "[mock] Örnek kampanya"},
	EN: Copy{Line: "[mock] This is a sample reply; with a live model the text follows your brief.", Head: "[mock] Sample campaign"},
}

// QuestionDSL is the filter ProposeSegment returns when the marketer answered the mock's question
// with its second option (customers who ordered in the last 30 days); see Question.
const QuestionDSL = `orders.some(o => o.orderDate >= ago("30d"))`

// Question is the one clarifying question the mock asks (D-088), for a brief that contains a
// question mark (see AsksQuestion): the first option keeps the filter of the brief's case, the
// second switches to QuestionDSL.
var Question = llm.Question{
	Text:    "[mock] Brief'te bir soru işareti var. Kitleyi hangi kapsamda alayım?",
	Options: []string{"Brief'teki kitle aynen kalsın", "Yalnızca son 30 günde alışveriş yapanlar"},
}

// AsksQuestion reports whether the mock asks its question for brief: the brief contains a question
// mark. The real model asks only when the audience is genuinely ambiguous; the mock needs a
// trigger the demo can type.
func AsksQuestion(brief string) bool { return strings.Contains(brief, "?") }

// Cases returns a copy of the hand-written cases: the sample brief first, then the 12 past
// campaigns of the stub data.
func Cases() []Case { return append([]Case(nil), cases...) }

// Default returns the case used for a brief without a hand-written answer.
func Default() Case { return unmatched }

// Match returns the case that answers brief, or the default case and false. The default case
// takes its title from the brief: "[mock] " and the start of the brief, within llm.MaxTitleChars.
func Match(brief string) (Case, bool) {
	n := normalize(brief)
	for _, c := range cases {
		if normalize(c.Brief) == n {
			return c, true
		}
	}
	c := unmatched
	if head := strings.Join(strings.Fields(brief), " "); head != "" {
		c.Title = truncateRunes(mockTag+" "+head, llm.MaxTitleChars)
	}
	return c, false
}

// truncateRunes cuts s to at most n characters, ending on a word when it can.
func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	cut := string([]rune(s)[:n])
	if i := strings.LastIndex(cut, " "); i > n/2 {
		cut = cut[:i]
	}
	return strings.TrimRightFunc(cut, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsPunct(r) })
}

// normalize lower-cases brief the Turkish way, unifies apostrophes, collapses white space and
// trims the punctuation around it, so that a brief typed with another spacing or a final full stop
// still matches.
func normalize(s string) string {
	s = strings.ToLowerSpecial(unicode.TurkishCase, s)
	s = strings.NewReplacer("’", "'", "‘", "'", "`", "'", "´", "'").Replace(s)
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimFunc(s, func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSpace(r) })
}
