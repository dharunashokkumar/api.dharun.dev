package api

import (
	"database/sql"
	"net/http"
	"net/url"
	"strings"
)

type phoneOperator struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type phoneCircle struct {
	Code     string  `json:"code"`
	Name     *string `json:"name"`
	Category *string `json:"category,omitempty"`
	Area     *string `json:"area,omitempty"`
}

type phoneView struct {
	Valid    bool   `json:"valid"`
	Problem  string `json:"problem,omitempty"`
	Type     string `json:"type"` // mobile, toll_free, premium or fixed_or_unknown
	E164     string `json:"e164,omitempty"`
	National string `json:"national,omitempty"`
	// Series, Operator and Circle describe who the number's series was first given to.
	Series   string         `json:"series,omitempty"`
	Operator *phoneOperator `json:"originalOperator,omitempty"`
	Circle   *phoneCircle   `json:"circle,omitempty"`
}

// nationalNumber strips what people put around an Indian number: spaces, dashes,
// brackets, a leading +91, 91, 0 or 0091.
func nationalNumber(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.LastIndex(raw, "+") > 0 {
		return "x" // a plus anywhere but in front
	}
	digits := strings.Map(func(r rune) rune {
		switch {
		case r >= '0' && r <= '9':
			return r
		case r == ' ' || r == '-' || r == '(' || r == ')' || r == '.' || r == '+':
			return -1
		}
		return 'x'
	}, raw)
	for _, prefix := range []string{"0091", "91", "0"} {
		if len(digits) > 10 && strings.HasPrefix(digits, prefix) {
			digits = strings.TrimPrefix(digits, prefix)
			break
		}
	}
	return digits
}

func (s *Server) handlePhone(w http.ResponseWriter, r *http.Request) {
	// A phone number points at a person: never logged, never shared-cached.
	info(r).private = true
	number := nationalNumber(r.PathValue("number"))
	view := phoneView{Type: "fixed_or_unknown"}
	switch {
	case strings.Contains(number, "x"):
		view.Problem = "A phone number has only digits, spaces, dashes, brackets and a leading +."
	case strings.HasPrefix(number, "1800") || strings.HasPrefix(number, "1860"):
		view.Type = "toll_free"
		if strings.HasPrefix(number, "1860") {
			view.Type = "shared_cost"
		}
		if len(number) < 10 || len(number) > 11 {
			view.Problem = "Toll-free numbers have 10 or 11 digits, such as 1800 123 4567."
		} else {
			view.Valid, view.National = true, number
		}
	case len(number) != 10:
		view.Problem = "An Indian phone number has 10 digits after the country code."
	case number[0] == '0':
		view.Problem = "After the country code or the leading 0, an Indian number does not start with 0."
	case number[0] >= '6' && number[0] <= '9':
		view.Type, view.Valid = "mobile", true
		view.E164 = "+91" + number
		view.National = number[:5] + " " + number[5:]
		view.Series = number[:4]
	default:
		// Fixed lines start with an area code of 2 to 4 digits that this API does not map.
		view.Valid, view.E164, view.National = true, "+91"+number, number
	}
	if view.Type == "mobile" {
		if set := s.data("phones"); set != nil {
			var operator, operatorName, circle, circleName, category, area sql.NullString
			err := set.row(`SELECT s.operator, o.name, s.circle, c.name, c.category, c.area FROM series s
				LEFT JOIN operators o ON o.code = s.operator LEFT JOIN circles c ON c.code = s.circle WHERE s.series = ?`, view.Series).
				Scan(&operator, &operatorName, &circle, &circleName, &category, &area)
			if err == nil {
				if operator.Valid && operatorName.Valid {
					view.Operator = &phoneOperator{operator.String, operatorName.String}
				}
				if circle.Valid {
					view.Circle = &phoneCircle{Code: circle.String, Name: str(circleName), Category: str(category), Area: str(area)}
				}
			}
		}
	}
	s.ok(w, r, view, meta{Source: s.source("phones"),
		Note: "Checks the number's form and names who its series was first given to. With number portability the current operator can differ, and whether the number is in use is not known."}, "private, no-store")
}

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name:        "check_indian_phone_number",
		Description: "Check an Indian phone number's form and type (mobile, toll-free, fixed line), format it, and name the operator and telecom circle its series was originally given to.",
		InputSchema: schema([]string{"number"}, map[string]any{"number": text("The phone number, with or without +91.")}),
		path: func(args map[string]any) (string, error) {
			number, err := required(args, "number")
			return "/v1/phone/" + url.PathEscape(number), err
		},
	})
}
