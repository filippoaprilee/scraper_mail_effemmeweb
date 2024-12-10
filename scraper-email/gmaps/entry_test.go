package gmaps_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/gosom/google-maps-scraper/gmaps"
)

func Test_EntryFromJSON(t *testing.T) {
	expected := gmaps.Entry{
		Title:    "Kipriakon",
		Category: "Restaurant",
		WebSite:  "",
		Phone:    "25 101555",
		CompleteAddress: gmaps.Address{
			Borough:    "",
			Street:     "Old port",
			City:       "Limassol",
			PostalCode: "3042",
			State:      "",
			Country:    "CY",
		},
		Emails: []string{}, // Modifica questo campo se vuoi testare email specifiche
	}

	raw, err := os.ReadFile("../testdata/raw.json")
	require.NoError(t, err)
	require.NotEmpty(t, raw)

	entry, err := gmaps.EntryFromJSON(raw)
	require.NoError(t, err)

	// Testa solo i campi di interesse
	require.Equal(t, expected.Title, entry.Title)
	require.Equal(t, expected.Category, entry.Category)
	require.Equal(t, expected.WebSite, entry.WebSite)
	require.Equal(t, expected.Phone, entry.Phone)
	require.Equal(t, expected.CompleteAddress, entry.CompleteAddress)
	require.Equal(t, expected.Emails, entry.Emails)
}

func Test_EntryFromJSON2(t *testing.T) {
	fnames := []string{
		"../testdata/panic.json",
		"../testdata/panic2.json",
	}
	for _, fname := range fnames {
		raw, err := os.ReadFile(fname)
		require.NoError(t, err)
		require.NotEmpty(t, raw)

		_, err = gmaps.EntryFromJSON(raw)
		require.NoError(t, err)
	}
}
