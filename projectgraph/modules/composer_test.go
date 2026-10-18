package modules

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
)

func TestComposerReadsThePSR4MapAndTheBins(t *testing.T) {
	root := fstest.MapFS{
		"composer.json":     {Data: []byte(`{"autoload": {"psr-4": {"App\\": "app/", "App\\Legacy\\": ["lib/", "old/"]}}, "autoload-dev": {"psr-4": {"Tests\\": "tests/"}}, "bin": ["bin/tool"]}`)},
		"pkg/composer.json": {Data: []byte(`{not json`)},
	}
	c := NewComposer(NewFiles([]string{"app/Kernel.php", "pkg/x.php"}), root)

	assert.Equal(t, []string{"app/Http/Kernel.php"}, c.Files(`App\Http\Kernel`))
	assert.Equal(t, []string{"lib/Mailer.php", "old/Mailer.php", "app/Legacy/Mailer.php"}, c.Files(`\App\Legacy\Mailer`), "the longest prefix comes first")
	assert.Equal(t, []string{"tests/UnitTest.php"}, c.Files(`Tests\UnitTest`))
	assert.Empty(t, c.Files(`Vendor\Thing`))
	assert.True(t, c.FirstParty(`App\Missing`))
	assert.True(t, c.FirstParty(`App`))
	assert.False(t, c.FirstParty(`Application\Thing`))
	assert.Equal(t, []string{"bin/tool"}, c.Bins())
	assert.Len(t, c.Problems(), 1)
	assert.Equal(t, "pkg/composer.json", c.Problems()[0].Path)
}
