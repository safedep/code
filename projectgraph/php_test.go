package projectgraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLastName(t *testing.T) {
	assert.Equal(t, "Kernel", lastName(`App\Http\Kernel`))
	assert.Equal(t, "Kernel", lastName(`\Kernel`))
	assert.Equal(t, "helper", lastName("helper"))
	assert.Empty(t, lastName(`App\`))
}
