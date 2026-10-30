package llm

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// QuestionLimitProblem is the problem a loop gives back to the model when it asks a question after
// the step used its MaxQuestionsPerStep questions (D-088). It costs a repair attempt, like any
// other validation failure (D-069).
const QuestionLimitProblem = "question limit reached for this step: decide now without asking; use the brief, the notes and the answers you already have"

// Shapes the summary rules reject: an HTML tag, a markdown link or image, a bare URL, a heading.
var (
	htmlTag      = regexp.MustCompile(`<\s*/?\s*[A-Za-z][^>]*>`)
	markdownLink = regexp.MustCompile(`\]\s*\(`)
	bareURL      = regexp.MustCompile(`(?i)\b(?:https?://|www\.)\S+`)
)

// CheckQuestion validates a clarifying question of the agent: a non-blank text of at most
// MaxQuestionChars characters; options, when given, MinOptions to MaxOptions distinct non-blank
// strings of at most MaxOptionChars characters each; no links, HTML or headings in either. It
// returns every problem, empty when the question is acceptable, and trims the fields in place.
func CheckQuestion(q *Question) []string {
	var problems []string
	q.Text = strings.TrimSpace(q.Text)
	if q.Text == "" {
		problems = append(problems, `"question" must not be blank`)
	} else if n := utf8.RuneCountInString(q.Text); n > MaxQuestionChars {
		problems = append(problems, fmt.Sprintf(`"question" is %d characters; at most %d`, n, MaxQuestionChars))
	}
	problems = append(problems, markdownProblems("question", q.Text)...)
	if q.Options != nil {
		if n := len(q.Options); n < MinOptions || n > MaxOptions {
			problems = append(problems, fmt.Sprintf(`"options" has %d items; give %d to %d, or none`, n, MinOptions, MaxOptions))
		}
		seen := map[string]bool{}
		for i := range q.Options {
			o := strings.TrimSpace(q.Options[i])
			q.Options[i] = o
			switch n := utf8.RuneCountInString(o); {
			case o == "":
				problems = append(problems, fmt.Sprintf(`"options[%d]" is blank`, i))
			case n > MaxOptionChars:
				problems = append(problems, fmt.Sprintf(`"options[%d]" is %d characters; at most %d`, i, n, MaxOptionChars))
			case seen[o]:
				problems = append(problems, fmt.Sprintf(`"options[%d]" repeats an earlier option`, i))
			}
			seen[o] = true
			problems = append(problems, markdownProblems(fmt.Sprintf("options[%d]", i), o)...)
		}
	}
	return problems
}

// CheckSummary validates the agent's markdown summary (D-088): non-blank, at most MaxSummaryChars
// characters, lists and bold allowed, no links, no HTML, no headings. It returns the trimmed
// summary and every problem.
func CheckSummary(markdown string) (string, []string) {
	markdown = strings.TrimSpace(markdown)
	var problems []string
	if markdown == "" {
		problems = append(problems, `"summary" must not be blank`)
	} else if n := utf8.RuneCountInString(markdown); n > MaxSummaryChars {
		problems = append(problems, fmt.Sprintf(`"summary" is %d characters; at most %d`, n, MaxSummaryChars))
	}
	problems = append(problems, markdownProblems("summary", markdown)...)
	for _, line := range strings.Split(markdown, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			problems = append(problems, `"summary" must not contain headings (a line starting with #)`)
			break
		}
	}
	return markdown, problems
}

// markdownProblems reports links, URLs and HTML in a field.
func markdownProblems(field, s string) []string {
	var problems []string
	if htmlTag.MatchString(s) {
		problems = append(problems, fmt.Sprintf(`"%s" must not contain HTML`, field))
	}
	if markdownLink.MatchString(s) || bareURL.MatchString(s) {
		problems = append(problems, fmt.Sprintf(`"%s" must not contain links or URLs`, field))
	}
	return problems
}

// CheckTitle validates a campaign title (D-088, campaign.title CHECK): one non-blank line of at most
// MaxTitleChars characters. It returns the trimmed title.
func CheckTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	switch n := utf8.RuneCountInString(title); {
	case title == "":
		return "", fmt.Errorf("the title is blank")
	case n > MaxTitleChars:
		return "", fmt.Errorf("the title is %d characters; at most %d", n, MaxTitleChars)
	case strings.ContainsAny(title, "\r\n"):
		return "", fmt.Errorf("the title must be one line")
	}
	return title, nil
}
