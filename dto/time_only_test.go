package dto

import "testing"

func Test_New(t *testing.T) {
	casesUint := []uint{
		0,
		10,
		20,
	}

	for _, caseUint := range casesUint {
		item := NewTimeOnly(caseUint)
		if uint(item) != caseUint {
			t.Errorf("caseUint=%d, want=%d, got=%d", caseUint, uint(item), item)
		}
	}

	casesInt := map[int]uint{
		0:  0,
		-5: 0,
		10: 10,
	}

	for caseInt, expected := range casesInt {
		item := NewTimeOnly(caseInt)
		if uint(item) != expected {
			t.Errorf("caseInt=%d, want=%d, got=%d", caseInt, expected, item)
		}
	}

	casesString := map[string]uint{
		"0":        0,
		"hello":    0,
		"15":       54_000,
		"15:15":    54_900,
		"15:15:15": 54_915,
		"::":       0,
		"1:2:3":    3_723,
		"01:02:03": 3_723,
		"_:_:_":    0,
		"":         0,
	}

	for caseString, expected := range casesString {
		item := NewTimeOnly(caseString)
		if uint(item) != expected {
			t.Errorf("caseString=%s, want=%d, got=%d", caseString, expected, item)
		}
	}
}

func Test_Hour(t *testing.T) {
	cases := map[string]uint{
		"0":        0,
		"15:00:05": 15,
		"00:03":    0,
	}

	for input, expected := range cases {
		item := NewTimeOnly(input)
		if item.Hour() != expected {
			t.Errorf("case=%s, want=%d, got=%d", input, expected, item)
		}
	}
}

func Test_Minute(t *testing.T) {
	cases := map[string]uint{
		"0":        0,
		"15:00:05": 0,
		"00:03":    3,
	}

	for input, expected := range cases {
		item := NewTimeOnly(input)
		if item.Minute() != expected {
			t.Errorf("case=%s, want=%d, got=%d", input, expected, item)
		}
	}
}

func Test_Second(t *testing.T) {
	cases := map[string]uint{
		"0":        0,
		"15:00:05": 5,
		"00:03":    0,
	}

	for input, expected := range cases {
		item := NewTimeOnly(input)
		if item.Second() != expected {
			t.Errorf("case=%s, want=%d, got=%d", input, expected, item)
		}
	}
}

func Test_String(t *testing.T) {
	cases := map[uint]string{
		54_000: "15:00:00",
		3_723:  "01:02:03",
		54_900: "15:15:00",
		54_915: "15:15:15",
	}

	for input, expected := range cases {
		item := NewTimeOnly(input)
		if item.String() != expected {
			t.Errorf("case=%d, want=%s, got=%s", input, expected, item)
		}
	}
}
