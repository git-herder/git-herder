// Сопоставление --qp и --qn: регэксп, иначе вхождение подстроки.
package workspace

import (
	"regexp"
	"strings"
)

func passFilters(c Copy, paths, names []string) bool {
	parent := c.Path()
	name := c.Name()
	for _, p := range paths {
		if !matchText(p, parent) {
			return false
		}
	}
	for _, n := range names {
		if !matchText(n, name) {
			return false
		}
	}
	return true
}

func matchText(pattern, value string) bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return strings.Contains(value, pattern)
	}
	return re.MatchString(value)
}
