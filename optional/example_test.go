package optional_test

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/imbrooklyn/shuttle/optional"
)

func ExampleOptional_Map() {
	name := optional.Some("  Brooklyn  ").
		Map(strings.TrimSpace).
		Filter(func(value string) bool { return value != "" }).
		Map(strings.ToUpper).
		OrElse("UNKNOWN")

	fmt.Println(name)
	// Output: BROOKLYN
}

func ExampleOptional_MarshalJSON() {
	presentNil := optional.Some[*int](nil)
	data, _ := json.Marshal(presentNil)

	var decoded optional.Optional[*int]
	_ = json.Unmarshal(data, &decoded)

	fmt.Println(presentNil.IsSome(), string(data), decoded.IsNone())
	// Output: true null true
}
