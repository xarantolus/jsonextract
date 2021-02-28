// Package jsonextract implements functions for finding and extracting any valid JavaScript object (not just JSON) from an io.Reader.
//
// This is an example of valid input for this package:
//     <script>
//     var x = {
//     	// Keys without quotes are valid in JavaScript, but not in JSON
//     	key: "value",
//     	num: 295.2,
//
//     	// Comments are removed while processing
//
//     	// Mixing normal and quoted keys is possible
//     	"obj": {
//     		"quoted": 325,
//     		'other quotes': true,
//     		unquoted: 'test', // This trailing comma will be removed
//     	},
//
//     	// JSON doesn't support all these number formats
//     	"dec": +21,
//     	"hex": 0x15,
//     	"oct": 0o25,
//     	"bin": 0b10101,
//     	bigint: 21n,
//
//     	// NaN will be converted to null. Infinity values are however not supported
//     	"num2": NaN,
//
//     	// Undefined will be interpreted as null
//     	"udef": undefined,
//
//     	`lastvalue`: `multiline strings are
//     no problem`
//     }
//     </script>
//
// The input will be searched for anything that looks like JavaScript notation. Found objects and arrays are
// converted to JSON, which can then be used for decoding into Go structures.
//
// Objects is a high-level function for easily extracting certain objects no matter their position within any other object.
// Reader is a lower-level function that gives you more control over how you process objects and arrays.
package jsonextract
