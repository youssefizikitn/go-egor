package config

// Represents a Competitive programming task's input and output for a some test case.
type TestCase struct {
	Input  string
	Output string
}

// The type of the input, can be "stdin", "stdout" or maybe file-based
// See Google Code Jam for file based IOTypes.
type IOType struct {
	Type string
}

// Specific to Java, Gives the name of the class and the computed task name.
type LanguageDescription struct {
	MainClass string
	TaskClass string
}

// Task representation
// Struct mapping to the json object received by competitive companion.
type Task struct {
	Name        string
	Group       string
	Url         string
	Interactive bool
	MemoryLimit float64
	TimeLimit   float64
	Tests       []TestCase
	TestType    string
	Input       IOType
	Output      IOType
	Languages   map[string]LanguageDescription
}
