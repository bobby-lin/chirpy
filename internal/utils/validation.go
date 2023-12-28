package utils

import (
	"strings"
)

func FilterWords(body string) (string, bool) {
	const filtered = "****"
	disallowedList := []string{"kerfuffle", "sharbert", "fornax"}
	isFiltered := false
	bodyWords := strings.Split(body, " ")

	for i, w := range bodyWords {
		for _, fw := range disallowedList {
			if strings.ToLower(w) == fw {
				bodyWords[i] = filtered
				isFiltered = true
			}
		}
	}

	finalBody := strings.Join(bodyWords, " ")

	return finalBody, isFiltered
}
