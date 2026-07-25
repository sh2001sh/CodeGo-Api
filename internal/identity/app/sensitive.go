package app

import requestsettings "github.com/sh2001sh/CodeGo-Api/internal/platform/requestsettings"

func CheckSensitiveText(text string) (bool, []string) {
	return SensitiveWordContains(text)
}

func SensitiveWordContains(text string) (bool, []string) {
	if len(requestsettings.SensitiveWords) == 0 {
		return false, nil
	}
	if len(text) == 0 {
		return false, nil
	}
	return AcSearchLower(text, requestsettings.SensitiveWords)
}

func AcSearchLower(text string, words []string) (bool, []string) {
	checkText := []rune(lowerString(text))
	hits := getOrBuildIdentityAC(words).MultiPatternSearch(checkText, true)
	if len(hits) == 0 {
		return false, nil
	}
	result := make([]string, 0, len(hits))
	for _, hit := range hits {
		result = append(result, string(hit.Word))
	}
	return true, result
}
