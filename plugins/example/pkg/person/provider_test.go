package person

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/entity"
)

// testPerson returns a fully populated person used as the old copy in the
// Changed tests
func testPerson() *Person {
	return &Person{
		FirstName: "Ada",
		LastName:  "Lovelace",
		Age:       36,
		Email:     "ada@example.com",
	}
}

func TestPersonChangedReplacesOnFirstNameChange(t *testing.T) {
	provider := &ExampleProvider{}
	old := testPerson()
	new := testPerson()
	new.FirstName = "Augusta"

	change, err := provider.Changed(context.Background(), old, new, nil, nil)
	require.NoError(t, err)
	require.Equal(t, entity.Replace, change)
}

func TestPersonChangedReplacesOnLastNameChange(t *testing.T) {
	provider := &ExampleProvider{}
	old := testPerson()
	new := testPerson()
	new.LastName = "King"

	change, err := provider.Changed(context.Background(), old, new, nil, nil)
	require.NoError(t, err)
	require.Equal(t, entity.Replace, change)
}

func TestPersonChangedUpdatesOnEmailChange(t *testing.T) {
	provider := &ExampleProvider{}
	old := testPerson()
	new := testPerson()
	new.Email = "countess@example.com"

	change, err := provider.Changed(context.Background(), old, new, nil, nil)
	require.NoError(t, err)
	require.Equal(t, entity.Update, change)
}

func TestPersonChangedReportsNoChangeForIdenticalPerson(t *testing.T) {
	provider := &ExampleProvider{}
	old := testPerson()
	new := testPerson()

	change, err := provider.Changed(context.Background(), old, new, nil, nil)
	require.NoError(t, err)
	require.Equal(t, entity.NoChange, change)
}
