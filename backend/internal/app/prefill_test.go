package app

import "testing"

func TestFormattedMilitaryNameUsesLastFirstMI(t *testing.T) {
	name := formattedMilitaryName(LaunchClaims{
		FullName:      "Jane Example",
		FirstName:     "Jane",
		LastName:      "Example",
		MiddleInitial: "q",
	})

	if name != "Example, Jane Q." {
		t.Fatalf("expected Last, First MI format, got %q", name)
	}
}

func TestDocuSealPrefillValuesNameRankOrdering(t *testing.T) {
	values := docusealPrefillValues(User{
		FullName:      "Example, Jane Q.",
		FirstName:     "Jane",
		LastName:      "Example",
		MiddleInitial: "Q",
		Rank:          "SGT",
		PayGrade:      "E-5",
		DoDID:         "1234567890",
		Email:         "jane.example@example.mil",
		UIC:           "WABC12",
	})

	nameFirstFields := []string{"Name & Rank", "Name / Rank", "Name Rank", "Name and Rank"}
	for _, field := range nameFirstFields {
		if values[field] != "Example, Jane Q. SGT" {
			t.Fatalf("expected %q to be name then rank, got %q", field, values[field])
		}
	}

	rankFirstFields := []string{"Rank & Name", "Rank / Name", "Rank Name", "Rank and Name"}
	for _, field := range rankFirstFields {
		if values[field] != "SGT Example, Jane Q." {
			t.Fatalf("expected %q to be rank then name, got %q", field, values[field])
		}
	}

	nameAliasFields := []string{"Operator Name", "Commander Name", "Commander's Name"}
	for _, field := range nameAliasFields {
		if values[field] != "Example, Jane Q." {
			t.Fatalf("expected %q to use formatted name, got %q", field, values[field])
		}
	}
}
