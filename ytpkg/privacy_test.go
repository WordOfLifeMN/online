package ytpkg

import (
	"testing"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/stretchr/testify/assert"
)

func TestPrivacyForView_NewItems(t *testing.T) {
	for view, expected := range map[catalog.View]Privacy{
		catalog.Public:  PrivacyPublic,
		catalog.Private: PrivacyUnlisted,
		catalog.Partner: PrivacyUnlisted,
	} {
		privacy, err := PrivacyForView(view)
		assert.NoError(t, err)
		assert.Equal(t, expected, privacy, "view %s", string(view))
	}
}

func TestPrivacyForView_RawIsRefused(t *testing.T) {
	_, err := PrivacyForView(catalog.Raw)
	assert.ErrorContains(t, err, "livestream")
}

func TestPrivacyForView_UnknownIsRefused(t *testing.T) {
	_, err := PrivacyForView(catalog.UnknownView)
	assert.Error(t, err)
}

func TestIsPrivacyConsistent(t *testing.T) {
	cases := []struct {
		view       catalog.View
		live       Privacy
		consistent bool
	}{
		{catalog.Public, PrivacyPublic, true},
		{catalog.Public, PrivacyUnlisted, false},
		{catalog.Public, PrivacyPrivate, false},

		{catalog.Private, PrivacyPrivate, true},
		{catalog.Private, PrivacyUnlisted, true},
		{catalog.Private, PrivacyPublic, false},

		{catalog.Partner, PrivacyUnlisted, true},
		{catalog.Partner, PrivacyPrivate, false},
		{catalog.Partner, PrivacyPublic, false},

		{catalog.Raw, PrivacyPrivate, false},
	}

	for _, c := range cases {
		assert.Equal(t, c.consistent, IsPrivacyConsistent(c.view, c.live),
			"sheet %s, YouTube %s", string(c.view), string(c.live))
	}
}

func TestResolvePrivacy(t *testing.T) {
	cases := []struct {
		view     catalog.View
		live     Privacy
		expected Privacy
	}{
		// new items take the mapping
		{catalog.Public, "", PrivacyPublic},
		{catalog.Private, "", PrivacyUnlisted},
		{catalog.Partner, "", PrivacyUnlisted},

		// consistent live privacy is kept
		{catalog.Private, PrivacyPrivate, PrivacyPrivate},
		{catalog.Private, PrivacyUnlisted, PrivacyUnlisted},

		// inconsistent live privacy is corrected
		{catalog.Partner, PrivacyPrivate, PrivacyUnlisted},
		{catalog.Public, PrivacyUnlisted, PrivacyPublic},
		{catalog.Private, PrivacyPublic, PrivacyUnlisted},
	}

	for _, c := range cases {
		privacy, err := ResolvePrivacy(c.view, c.live)
		assert.NoError(t, err)
		assert.Equal(t, c.expected, privacy, "sheet %s, YouTube %q", string(c.view), string(c.live))
	}

	_, err := ResolvePrivacy(catalog.Raw, PrivacyPrivate)
	assert.Error(t, err, "raw is refused even when YouTube agrees it is private")
}
