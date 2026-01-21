package bstruct

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

type Animal struct {
	Type string `union:"@discriminator" json:"type"`
	*Dog `       union:"dog,@default"`
	*Cat `       union:"cat"`
}

type Dog struct {
	Breed string `json:"breed"`
	Bark  string `json:"bark"`
}

type Cat struct {
	Color string `json:"color"`
	Meow  string `json:"meow"`
}

func TestUnmarshalUnion(t *testing.T) {
	t.Run("Unmarshal Dog", func(t *testing.T) {
		jsonData := `{"type": "dog", "breed": "Labrador", "bark": "Woof!"}`

		var animal Animal
		err := UnmarshalUnion([]byte(jsonData), &animal)

		require.NoError(t, err)
		require.Equal(t, "dog", animal.Type)
		require.NotNil(t, animal.Dog)
		require.Nil(t, animal.Cat)
		require.Equal(t, "Labrador", animal.Breed)
		require.Equal(t, "Woof!", animal.Bark)
	})

	t.Run("Unmarshal Cat", func(t *testing.T) {
		jsonData := `{"type": "cat", "color": "Tabby", "meow": "Meow!"}`

		var animal Animal
		err := UnmarshalUnion([]byte(jsonData), &animal)

		require.NoError(t, err)
		require.Equal(t, "cat", animal.Type)
		require.NotNil(t, animal.Cat)
		require.Nil(t, animal.Dog)
		require.Equal(t, "Tabby", animal.Color)
		require.Equal(t, "Meow!", animal.Meow)
	})

	t.Run("Unmarshal @default", func(t *testing.T) {
		jsonData := `{"breed": "Labrador", "bark": "Woof!"}`

		var animal Animal
		err := UnmarshalUnion([]byte(jsonData), &animal)

		require.NoError(t, err)
		require.Equal(t, "dog", animal.Type)
		require.NotNil(t, animal.Dog)
		require.Nil(t, animal.Cat)
		require.Equal(t, "Labrador", animal.Breed)
		require.Equal(t, "Woof!", animal.Bark)
	})

	t.Run("Invalid Discriminator", func(t *testing.T) {
		jsonData := `{"type": "bird", "species": "Parrot"}`

		var animal Animal
		err := UnmarshalUnion([]byte(jsonData), &animal)

		require.Error(t, err)
		require.Contains(t, err.Error(), "no matching union field found for type=bird")
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		jsonData := `{"type": "dog", "breed": "Labrador", "bark": "Woof!"`
		var animal Animal
		err := UnmarshalUnion([]byte(jsonData), &animal)
		require.Error(t, err)
		require.IsType(t, &json.SyntaxError{}, err)
	})
}
