package color

const (
	Green  = "\033[92m"
	Yellow = "\033[93m"
	Red    = "\033[91m"
	Gray   = "\033[90m"
	End    = "\033[0m"
)

func GreenText(s string) string {
	return Green + s + End
}

func YellowText(s string) string {
	return Yellow + s + End
}

func RedText(s string) string {
	return Red + s + End
}

func MutedText(s string) string {
	return Gray + s + End
}
