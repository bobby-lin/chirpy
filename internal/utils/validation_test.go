package utils

import (
	"testing"
)

type filterWordsTest struct {
	body               string
	expectedReturnBody string
	expectedIsFiltered bool
}

var filterWordsTests = []filterWordsTest{
	{
		body:               "lorem ipsum dolor",
		expectedReturnBody: "lorem ipsum dolor",
		expectedIsFiltered: false,
	},
	{
		body:               "kerfuffle",
		expectedReturnBody: "****",
		expectedIsFiltered: true,
	},
	{
		body:               "this is kerfuffle man!",
		expectedReturnBody: "this is **** man!",
		expectedIsFiltered: true,
	},
	{
		body:               "this is KERFUFFLE man!",
		expectedReturnBody: "this is **** man!",
		expectedIsFiltered: true,
	},
	{
		body:               "i love kerfuffle!",
		expectedReturnBody: "i love kerfuffle!",
		expectedIsFiltered: false,
	}, {
		body:               "Sharbert love kerfuffle man",
		expectedReturnBody: "**** love **** man",
		expectedIsFiltered: true,
	},
}

func TestFilterWords(t *testing.T) {
	for _, test := range filterWordsTests {
		returnBody, isFiltered := FilterWords(test.body)
		if isFiltered != test.expectedIsFiltered {
			t.Errorf("Output %t not equal to expected %t", isFiltered, test.expectedIsFiltered)
		}

		if returnBody != test.expectedReturnBody {
			t.Errorf("Output %s not equal to expected %s", returnBody, test.expectedReturnBody)
		}

	}
}
