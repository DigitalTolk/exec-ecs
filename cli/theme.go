package cli

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Theme struct {
	Name                 string
	TitleStyle           lipgloss.Style
	ItemStyle            lipgloss.Style
	ItemStyleAlt         lipgloss.Style
	SelectedItem         lipgloss.Style
	FilterStyle          lipgloss.Style
	HelpStyle            lipgloss.Style
	MainBg               lipgloss.Color
	MainBorder           lipgloss.Color
	StatusBg             lipgloss.Color
	StatusFg             lipgloss.Color
	TitleBg              lipgloss.Color
	TitleFg              lipgloss.Color
	BorderStyle          lipgloss.Border
	SelectionIcon        string
	UnselectedIcon       string
	LoadingHint          string
	SpinnerCharset       []string
	SelectedPaddingRight int
	MenuAlignment        lipgloss.Position
	HelpHint             string
}

var (
	SimpleCIDETheme = &Theme{
		Name:                 "Turbo C++",
		TitleStyle:           lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("4")).Align(lipgloss.Center),
		ItemStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("17")).PaddingLeft(2),
		ItemStyleAlt:         lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("18")).PaddingLeft(2),
		SelectedItem:         lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("14")).Bold(true).Border(lipgloss.NormalBorder(), true).BorderForeground(lipgloss.Color("14")).PaddingLeft(2).PaddingRight(4),
		FilterStyle:          lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("14")).PaddingLeft(2).MarginTop(1).MarginBottom(1),
		HelpStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("4")).MarginTop(1).Padding(0, 1),
		MainBg:               lipgloss.Color("17"),
		MainBorder:           lipgloss.Color("4"),
		StatusBg:             lipgloss.Color("4"),
		StatusFg:             lipgloss.Color("15"),
		TitleBg:              lipgloss.Color("4"),
		TitleFg:              lipgloss.Color("15"),
		BorderStyle:          lipgloss.NormalBorder(),
		SelectionIcon:        ">",
		UnselectedIcon:       " ",
		LoadingHint:          "Loading...",
		SpinnerCharset:       []string{"-", "\\", "|", "/"},
		SelectedPaddingRight: 4,
		MenuAlignment:        lipgloss.Center,
		HelpHint:             "C++",
	}
	ModernPastelTheme = &Theme{
		Name:                 "Modern Pastel",
		TitleStyle:           lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#eaeaea")).Background(lipgloss.Color("#A7C7E7")).Align(lipgloss.Center),
		ItemStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#eaeaea")).Background(lipgloss.Color("#232946")).PaddingLeft(2),
		ItemStyleAlt:         lipgloss.NewStyle().Foreground(lipgloss.Color("#eaeaea")).Background(lipgloss.Color("#2d3250")).PaddingLeft(2),
		SelectedItem:         lipgloss.NewStyle().Foreground(lipgloss.Color("#232946")).Background(lipgloss.Color("#ffd6e0")).Bold(true).Border(lipgloss.DoubleBorder(), true).BorderForeground(lipgloss.Color("#ffd6e0")).PaddingLeft(2).PaddingRight(4),
		FilterStyle:          lipgloss.NewStyle().Foreground(lipgloss.Color("#232946")).Background(lipgloss.Color("#FFF5BA")).PaddingLeft(2).MarginTop(1).MarginBottom(1),
		HelpStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#eaeaea")).Background(lipgloss.Color("#C7CEEA")).MarginTop(1).Padding(0, 1),
		MainBg:               lipgloss.Color("#232946"),
		MainBorder:           lipgloss.Color("#A7C7E7"),
		StatusBg:             lipgloss.Color("#C7CEEA"),
		StatusFg:             lipgloss.Color("#eaeaea"),
		TitleBg:              lipgloss.Color("#A7C7E7"),
		TitleFg:              lipgloss.Color("#eaeaea"),
		BorderStyle:          lipgloss.DoubleBorder(),
		SelectionIcon:        "▶",
		UnselectedIcon:       " ",
		LoadingHint:          "Connecting to the cloud...",
		SpinnerCharset:       []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		SelectedPaddingRight: 4,
		MenuAlignment:        lipgloss.Center,
		HelpHint:             "",
	}
	GhostyTheme = &Theme{
		Name:                 "Ghosty",
		TitleStyle:           lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#e0e6f0")).Background(lipgloss.Color("#6c6f93")).Align(lipgloss.Center).Italic(true).Underline(true),
		ItemStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#e0e6f0")).Background(lipgloss.Color("#232946")).PaddingLeft(2),
		ItemStyleAlt:         lipgloss.NewStyle().Foreground(lipgloss.Color("#e0e6f0")).Background(lipgloss.Color("#282a36")).PaddingLeft(2),
		SelectedItem:         lipgloss.NewStyle().Foreground(lipgloss.Color("#232946")).Background(lipgloss.Color("#b8c0ff")).Bold(true).Border(lipgloss.RoundedBorder(), true).BorderForeground(lipgloss.Color("#b8c0ff")).PaddingLeft(2).PaddingRight(6).Italic(true),
		FilterStyle:          lipgloss.NewStyle().Foreground(lipgloss.Color("#232946")).Background(lipgloss.Color("#e0e6f0")).PaddingLeft(2).MarginTop(1).MarginBottom(1).Italic(true),
		HelpStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#e0e6f0")).Background(lipgloss.Color("#6c6f93")).MarginTop(1).Padding(0, 1).Italic(true),
		MainBg:               lipgloss.Color("#232946"),
		MainBorder:           lipgloss.Color("#b8c0ff"),
		StatusBg:             lipgloss.Color("#6c6f93"),
		StatusFg:             lipgloss.Color("#e0e6f0"),
		TitleBg:              lipgloss.Color("#6c6f93"),
		TitleFg:              lipgloss.Color("#e0e6f0"),
		BorderStyle:          lipgloss.RoundedBorder(),
		SelectionIcon:        "👻",
		UnselectedIcon:       " ",
		LoadingHint:          "Summoning ghosts...",
		SpinnerCharset:       []string{"🌫️ ", "👻 ", "🌫️ ", " ", " ", " ", " ", " ", " ", " "},
		SelectedPaddingRight: 6,
		MenuAlignment:        lipgloss.Center,
		HelpHint:             "👻",
	}
	// 1. Solarized Dark
	SolarizedDarkTheme = &Theme{
		Name:                 "Solarized Dark",
		TitleStyle:           lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#93a1a1")).Background(lipgloss.Color("#073642")).Align(lipgloss.Center),
		ItemStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#93a1a1")).Background(lipgloss.Color("#002b36")).PaddingLeft(2),
		ItemStyleAlt:         lipgloss.NewStyle().Foreground(lipgloss.Color("#93a1a1")).Background(lipgloss.Color("#073642")).PaddingLeft(2),
		SelectedItem:         lipgloss.NewStyle().Foreground(lipgloss.Color("#002b36")).Background(lipgloss.Color("#b58900")).Bold(true).Border(lipgloss.NormalBorder(), true).BorderForeground(lipgloss.Color("#b58900")).PaddingLeft(2).PaddingRight(4),
		FilterStyle:          lipgloss.NewStyle().Foreground(lipgloss.Color("#002b36")).Background(lipgloss.Color("#b58900")).PaddingLeft(2).MarginTop(1).MarginBottom(1),
		HelpStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#93a1a1")).Background(lipgloss.Color("#073642")).MarginTop(1).Padding(0, 1),
		MainBg:               lipgloss.Color("#002b36"),
		MainBorder:           lipgloss.Color("#b58900"),
		StatusBg:             lipgloss.Color("#073642"),
		StatusFg:             lipgloss.Color("#93a1a1"),
		TitleBg:              lipgloss.Color("#073642"),
		TitleFg:              lipgloss.Color("#93a1a1"),
		BorderStyle:          lipgloss.NormalBorder(),
		SelectionIcon:        "⦿",
		UnselectedIcon:       " ",
		LoadingHint:          "Solarizing...",
		SpinnerCharset:       []string{"☀", "☼", "☀", "☼"},
		SelectedPaddingRight: 4,
		MenuAlignment:        lipgloss.Center,
		HelpHint:             "",
	}
	// 2. Dracula
	DraculaTheme = &Theme{
		Name:                 "Dracula",
		TitleStyle:           lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f8f8f2")).Background(lipgloss.Color("#282a36")).Align(lipgloss.Center),
		ItemStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#f8f8f2")).Background(lipgloss.Color("#282a36")).PaddingLeft(2),
		ItemStyleAlt:         lipgloss.NewStyle().Foreground(lipgloss.Color("#f8f8f2")).Background(lipgloss.Color("#44475a")).PaddingLeft(2),
		SelectedItem:         lipgloss.NewStyle().Foreground(lipgloss.Color("#282a36")).Background(lipgloss.Color("#bd93f9")).Bold(true).Border(lipgloss.DoubleBorder(), true).BorderForeground(lipgloss.Color("#bd93f9")).PaddingLeft(2).PaddingRight(4),
		FilterStyle:          lipgloss.NewStyle().Foreground(lipgloss.Color("#282a36")).Background(lipgloss.Color("#bd93f9")).PaddingLeft(2).MarginTop(1).MarginBottom(1),
		HelpStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#f8f8f2")).Background(lipgloss.Color("#282a36")).MarginTop(1).Padding(0, 1),
		MainBg:               lipgloss.Color("#282a36"),
		MainBorder:           lipgloss.Color("#bd93f9"),
		StatusBg:             lipgloss.Color("#282a36"),
		StatusFg:             lipgloss.Color("#f8f8f2"),
		TitleBg:              lipgloss.Color("#282a36"),
		TitleFg:              lipgloss.Color("#f8f8f2"),
		BorderStyle:          lipgloss.DoubleBorder(),
		SelectionIcon:        "🦇",
		UnselectedIcon:       " ",
		LoadingHint:          "Awakening Dracula...",
		SpinnerCharset:       []string{"🦇", " ", "🦇", " "},
		SelectedPaddingRight: 4,
		MenuAlignment:        lipgloss.Center,
		HelpHint:             "🦇",
	}
	// 3. Pac-Man
	PacManTheme = &Theme{
		Name:                 "Pac-Man",
		TitleStyle:           lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#fff200")).Background(lipgloss.Color("#000000")).Align(lipgloss.Center),
		ItemStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#fff200")).Background(lipgloss.Color("#000000")).PaddingLeft(2),
		ItemStyleAlt:         lipgloss.NewStyle().Foreground(lipgloss.Color("#fff200")).Background(lipgloss.Color("#22223b")).PaddingLeft(2),
		SelectedItem:         lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#fff200")).Bold(true).Border(lipgloss.NormalBorder(), true).BorderForeground(lipgloss.Color("#fff200")).PaddingLeft(2).PaddingRight(6),
		FilterStyle:          lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#fff200")).PaddingLeft(2).MarginTop(1).MarginBottom(1),
		HelpStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#fff200")).Background(lipgloss.Color("#000000")).MarginTop(1).Padding(0, 1),
		MainBg:               lipgloss.Color("#000000"),
		MainBorder:           lipgloss.Color("#fff200"),
		StatusBg:             lipgloss.Color("#000000"),
		StatusFg:             lipgloss.Color("#fff200"),
		TitleBg:              lipgloss.Color("#000000"),
		TitleFg:              lipgloss.Color("#fff200"),
		BorderStyle:          lipgloss.NormalBorder(),
		SelectionIcon:        "🟡",
		UnselectedIcon:       " ",
		LoadingHint:          "Eating dots...",
		SpinnerCharset:       []string{"C", "c", "o", ".", " ", ".", "o", "c", "C"},
		SelectedPaddingRight: 6,
		MenuAlignment:        lipgloss.Center,
		HelpHint:             "🟡",
	}
	// 4. Matrix
	MatrixTheme = &Theme{
		Name:                 "Matrix",
		TitleStyle:           lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00ff41")).Background(lipgloss.Color("#000000")).Align(lipgloss.Center),
		ItemStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff41")).Background(lipgloss.Color("#000000")).PaddingLeft(2),
		ItemStyleAlt:         lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff41")).Background(lipgloss.Color("#22223b")).PaddingLeft(2),
		SelectedItem:         lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#00ff41")).Bold(true).Border(lipgloss.NormalBorder(), true).BorderForeground(lipgloss.Color("#00ff41")).PaddingLeft(2).PaddingRight(6),
		FilterStyle:          lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#00ff41")).PaddingLeft(2).MarginTop(1).MarginBottom(1),
		HelpStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff41")).Background(lipgloss.Color("#000000")).MarginTop(1).Padding(0, 1),
		MainBg:               lipgloss.Color("#000000"),
		MainBorder:           lipgloss.Color("#00ff41"),
		StatusBg:             lipgloss.Color("#000000"),
		StatusFg:             lipgloss.Color("#00ff41"),
		TitleBg:              lipgloss.Color("#000000"),
		TitleFg:              lipgloss.Color("#00ff41"),
		BorderStyle:          lipgloss.NormalBorder(),
		SelectionIcon:        "▣",
		UnselectedIcon:       " ",
		LoadingHint:          "Following the white rabbit...",
		SpinnerCharset:       []string{"|", "/", "-", "\\"},
		SelectedPaddingRight: 6,
		MenuAlignment:        lipgloss.Center,
		HelpHint:             "▣",
	}
	// 5. Gameboy
	GameboyTheme = &Theme{
		Name:                 "Gameboy",
		TitleStyle:           lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#9bbc0f")).Background(lipgloss.Color("#0f380f")).Align(lipgloss.Center),
		ItemStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#9bbc0f")).Background(lipgloss.Color("#0f380f")).PaddingLeft(2),
		ItemStyleAlt:         lipgloss.NewStyle().Foreground(lipgloss.Color("#9bbc0f")).Background(lipgloss.Color("#306230")).PaddingLeft(2),
		SelectedItem:         lipgloss.NewStyle().Foreground(lipgloss.Color("#0f380f")).Background(lipgloss.Color("#9bbc0f")).Bold(true).Border(lipgloss.NormalBorder(), true).BorderForeground(lipgloss.Color("#9bbc0f")).PaddingLeft(2).PaddingRight(4),
		FilterStyle:          lipgloss.NewStyle().Foreground(lipgloss.Color("#0f380f")).Background(lipgloss.Color("#9bbc0f")).PaddingLeft(2).MarginTop(1).MarginBottom(1),
		HelpStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#9bbc0f")).Background(lipgloss.Color("#0f380f")).MarginTop(1).Padding(0, 1),
		MainBg:               lipgloss.Color("#0f380f"),
		MainBorder:           lipgloss.Color("#9bbc0f"),
		StatusBg:             lipgloss.Color("#0f380f"),
		StatusFg:             lipgloss.Color("#9bbc0f"),
		TitleBg:              lipgloss.Color("#0f380f"),
		TitleFg:              lipgloss.Color("#9bbc0f"),
		BorderStyle:          lipgloss.NormalBorder(),
		SelectionIcon:        "♥",
		UnselectedIcon:       " ",
		LoadingHint:          "Blowing cartridge...",
		SpinnerCharset:       []string{"░", "▒", "▓", "█"},
		SelectedPaddingRight: 4,
		MenuAlignment:        lipgloss.Center,
		HelpHint:             "♥",
	}
	// 6. DOS ANSI
	DOSANSITheme = &Theme{
		Name:                 "DOS ANSI",
		TitleStyle:           lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("226")).Background(lipgloss.Color("19")).Align(lipgloss.Center),
		ItemStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Background(lipgloss.Color("19")).PaddingLeft(2),
		ItemStyleAlt:         lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Background(lipgloss.Color("21")).PaddingLeft(2),
		SelectedItem:         lipgloss.NewStyle().Foreground(lipgloss.Color("19")).Background(lipgloss.Color("226")).Bold(true).Border(lipgloss.NormalBorder(), true).BorderForeground(lipgloss.Color("226")).PaddingLeft(2).PaddingRight(4),
		FilterStyle:          lipgloss.NewStyle().Foreground(lipgloss.Color("19")).Background(lipgloss.Color("226")).PaddingLeft(2).MarginTop(1).MarginBottom(1),
		HelpStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Background(lipgloss.Color("19")).MarginTop(1).Padding(0, 1),
		MainBg:               lipgloss.Color("19"),
		MainBorder:           lipgloss.Color("226"),
		StatusBg:             lipgloss.Color("19"),
		StatusFg:             lipgloss.Color("226"),
		TitleBg:              lipgloss.Color("19"),
		TitleFg:              lipgloss.Color("226"),
		BorderStyle:          lipgloss.NormalBorder(),
		SelectionIcon:        "█",
		UnselectedIcon:       " ",
		LoadingHint:          "Booting up...",
		SpinnerCharset:       []string{"█", "▒", "░", " "},
		SelectedPaddingRight: 4,
		MenuAlignment:        lipgloss.Center,
		HelpHint:             "",
	}
	// 7. Cyberpunk
	CyberpunkTheme = &Theme{
		Name:                 "Cyberpunk",
		TitleStyle:           lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ff00c8")).Background(lipgloss.Color("#0f1021")).Align(lipgloss.Center),
		ItemStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#00fff7")).Background(lipgloss.Color("#0f1021")).PaddingLeft(2),
		ItemStyleAlt:         lipgloss.NewStyle().Foreground(lipgloss.Color("#ff00c8")).Background(lipgloss.Color("#232946")).PaddingLeft(2),
		SelectedItem:         lipgloss.NewStyle().Foreground(lipgloss.Color("#0f1021")).Background(lipgloss.Color("#ff00c8")).Bold(true).Border(lipgloss.DoubleBorder(), true).BorderForeground(lipgloss.Color("#ff00c8")).PaddingLeft(2).PaddingRight(6),
		FilterStyle:          lipgloss.NewStyle().Foreground(lipgloss.Color("#0f1021")).Background(lipgloss.Color("#ff00c8")).PaddingLeft(2).MarginTop(1).MarginBottom(1),
		HelpStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#00fff7")).Background(lipgloss.Color("#0f1021")).MarginTop(1).Padding(0, 1),
		MainBg:               lipgloss.Color("#0f1021"),
		MainBorder:           lipgloss.Color("#ff00c8"),
		StatusBg:             lipgloss.Color("#0f1021"),
		StatusFg:             lipgloss.Color("#00fff7"),
		TitleBg:              lipgloss.Color("#0f1021"),
		TitleFg:              lipgloss.Color("#ff00c8"),
		BorderStyle:          lipgloss.DoubleBorder(),
		SelectionIcon:        "⮞",
		UnselectedIcon:       " ",
		LoadingHint:          "Hacking the planet...",
		SpinnerCharset:       []string{"⠿", "⠾", "⠽", "⠻", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		SelectedPaddingRight: 6,
		MenuAlignment:        lipgloss.Center,
		HelpHint:             "⮞",
	}
	// 8. Zelda
	ZeldaTheme = &Theme{
		Name:                 "Zelda",
		TitleStyle:           lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f9d923")).Background(lipgloss.Color("#1e212b")).Align(lipgloss.Center),
		ItemStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#f9d923")).Background(lipgloss.Color("#1e212b")).PaddingLeft(2),
		ItemStyleAlt:         lipgloss.NewStyle().Foreground(lipgloss.Color("#f9d923")).Background(lipgloss.Color("#232946")).PaddingLeft(2),
		SelectedItem:         lipgloss.NewStyle().Foreground(lipgloss.Color("#1e212b")).Background(lipgloss.Color("#f9d923")).Bold(true).Border(lipgloss.RoundedBorder(), true).BorderForeground(lipgloss.Color("#f9d923")).PaddingLeft(2).PaddingRight(6),
		FilterStyle:          lipgloss.NewStyle().Foreground(lipgloss.Color("#1e212b")).Background(lipgloss.Color("#f9d923")).PaddingLeft(2).MarginTop(1).MarginBottom(1),
		HelpStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#f9d923")).Background(lipgloss.Color("#1e212b")).MarginTop(1).Padding(0, 1),
		MainBg:               lipgloss.Color("#1e212b"),
		MainBorder:           lipgloss.Color("#f9d923"),
		StatusBg:             lipgloss.Color("#1e212b"),
		StatusFg:             lipgloss.Color("#f9d923"),
		TitleBg:              lipgloss.Color("#1e212b"),
		TitleFg:              lipgloss.Color("#f9d923"),
		BorderStyle:          lipgloss.RoundedBorder(),
		SelectionIcon:        "💚",
		UnselectedIcon:       " ",
		LoadingHint:          "Finding the Triforce...",
		SpinnerCharset:       []string{"▲", "△", "▲", "△"},
		SelectedPaddingRight: 6,
		MenuAlignment:        lipgloss.Center,
		HelpHint:             "💚",
	}
	// 9. Mac Classic
	MacClassicTheme = &Theme{
		Name:                 "Mac Classic",
		TitleStyle:           lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#c0c0c0")).Align(lipgloss.Center),
		ItemStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#c0c0c0")).PaddingLeft(2),
		ItemStyleAlt:         lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#e0e0e0")).PaddingLeft(2),
		SelectedItem:         lipgloss.NewStyle().Foreground(lipgloss.Color("#c0c0c0")).Background(lipgloss.Color("#000000")).Bold(true).Border(lipgloss.NormalBorder(), true).BorderForeground(lipgloss.Color("#000000")).PaddingLeft(2).PaddingRight(4),
		FilterStyle:          lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#c0c0c0")).PaddingLeft(2).MarginTop(1).MarginBottom(1),
		HelpStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#c0c0c0")).MarginTop(1).Padding(0, 1),
		MainBg:               lipgloss.Color("#c0c0c0"),
		MainBorder:           lipgloss.Color("#000000"),
		StatusBg:             lipgloss.Color("#c0c0c0"),
		StatusFg:             lipgloss.Color("#000000"),
		TitleBg:              lipgloss.Color("#c0c0c0"),
		TitleFg:              lipgloss.Color("#000000"),
		BorderStyle:          lipgloss.NormalBorder(),
		SelectionIcon:        "",
		UnselectedIcon:       " ",
		LoadingHint:          "Welcome to Macintosh...",
		SpinnerCharset:       []string{"⌛", "⏳", "⌛", "⏳"},
		SelectedPaddingRight: 4,
		MenuAlignment:        lipgloss.Center,
		HelpHint:             "",
	}
	// 10. Commodore 64
	C64Theme = &Theme{
		Name:                 "Commodore 64",
		TitleStyle:           lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7869c4")).Background(lipgloss.Color("#40318d")).Align(lipgloss.Center),
		ItemStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#7869c4")).Background(lipgloss.Color("#40318d")).PaddingLeft(2),
		ItemStyleAlt:         lipgloss.NewStyle().Foreground(lipgloss.Color("#7869c4")).Background(lipgloss.Color("#5a4fcf")).PaddingLeft(2),
		SelectedItem:         lipgloss.NewStyle().Foreground(lipgloss.Color("#40318d")).Background(lipgloss.Color("#7869c4")).Bold(true).Border(lipgloss.NormalBorder(), true).BorderForeground(lipgloss.Color("#7869c4")).PaddingLeft(2).PaddingRight(4),
		FilterStyle:          lipgloss.NewStyle().Foreground(lipgloss.Color("#40318d")).Background(lipgloss.Color("#7869c4")).PaddingLeft(2).MarginTop(1).MarginBottom(1),
		HelpStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#7869c4")).Background(lipgloss.Color("#40318d")).MarginTop(1).Padding(0, 1),
		MainBg:               lipgloss.Color("#40318d"),
		MainBorder:           lipgloss.Color("#7869c4"),
		StatusBg:             lipgloss.Color("#40318d"),
		StatusFg:             lipgloss.Color("#7869c4"),
		TitleBg:              lipgloss.Color("#40318d"),
		TitleFg:              lipgloss.Color("#7869c4"),
		BorderStyle:          lipgloss.NormalBorder(),
		SelectionIcon:        "█",
		UnselectedIcon:       " ",
		LoadingHint:          "READY.",
		SpinnerCharset:       []string{"█", " ", "█", " "},
		SelectedPaddingRight: 4,
		MenuAlignment:        lipgloss.Center,
		HelpHint:             "READY.",
	}
	// 11. Synthwave '84
	SynthwaveTheme = &Theme{
		Name:                 "Synthwave '84",
		TitleStyle:           lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f5f5f5")).Background(lipgloss.Color("#ff5fd2")).Align(lipgloss.Center).Italic(true),
		ItemStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#f5f5f5")).Background(lipgloss.Color("#2b213a")).PaddingLeft(2),
		ItemStyleAlt:         lipgloss.NewStyle().Foreground(lipgloss.Color("#f5f5f5")).Background(lipgloss.Color("#3a2b5f")).PaddingLeft(2),
		SelectedItem:         lipgloss.NewStyle().Foreground(lipgloss.Color("#2b213a")).Background(lipgloss.Color("#ff5fd2")).Bold(true).Border(lipgloss.DoubleBorder(), true).BorderForeground(lipgloss.Color("#ff5fd2")).PaddingLeft(2).PaddingRight(6).Italic(true),
		FilterStyle:          lipgloss.NewStyle().Foreground(lipgloss.Color("#2b213a")).Background(lipgloss.Color("#ff5fd2")).PaddingLeft(2).MarginTop(1).MarginBottom(1),
		HelpStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#f5f5f5")).Background(lipgloss.Color("#ff5fd2")).MarginTop(1).Padding(0, 1).Italic(true),
		MainBg:               lipgloss.Color("#2b213a"),
		MainBorder:           lipgloss.Color("#ff5fd2"),
		StatusBg:             lipgloss.Color("#ff5fd2"),
		StatusFg:             lipgloss.Color("#f5f5f5"),
		TitleBg:              lipgloss.Color("#ff5fd2"),
		TitleFg:              lipgloss.Color("#f5f5f5"),
		BorderStyle:          lipgloss.DoubleBorder(),
		SelectionIcon:        "🌴",
		UnselectedIcon:       " ",
		LoadingHint:          "Booting up the DeLorean...",
		SpinnerCharset:       []string{"🕹️", "🌴", "🦩", "🌅"},
		SelectedPaddingRight: 6,
		MenuAlignment:        lipgloss.Center,
		HelpHint:             "Vaporwave mode",
	}
	// 12. Fairyfloss
	FairyflossTheme = &Theme{
		Name:                 "Fairyfloss",
		TitleStyle:           lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f8f8f2")).Background(lipgloss.Color("#ffb8d1")).Align(lipgloss.Center),
		ItemStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#f8f8f2")).Background(lipgloss.Color("#8c7dd1")).PaddingLeft(2),
		ItemStyleAlt:         lipgloss.NewStyle().Foreground(lipgloss.Color("#f8f8f2")).Background(lipgloss.Color("#ffd7ef")).PaddingLeft(2),
		SelectedItem:         lipgloss.NewStyle().Foreground(lipgloss.Color("#8c7dd1")).Background(lipgloss.Color("#ffb8d1")).Bold(true).Border(lipgloss.RoundedBorder(), true).BorderForeground(lipgloss.Color("#ffb8d1")).PaddingLeft(2).PaddingRight(6),
		FilterStyle:          lipgloss.NewStyle().Foreground(lipgloss.Color("#8c7dd1")).Background(lipgloss.Color("#ffb8d1")).PaddingLeft(2).MarginTop(1).MarginBottom(1),
		HelpStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#f8f8f2")).Background(lipgloss.Color("#ffb8d1")).MarginTop(1).Padding(0, 1),
		MainBg:               lipgloss.Color("#8c7dd1"),
		MainBorder:           lipgloss.Color("#ffb8d1"),
		StatusBg:             lipgloss.Color("#ffb8d1"),
		StatusFg:             lipgloss.Color("#f8f8f2"),
		TitleBg:              lipgloss.Color("#ffb8d1"),
		TitleFg:              lipgloss.Color("#f8f8f2"),
		BorderStyle:          lipgloss.RoundedBorder(),
		SelectionIcon:        "🧚",
		UnselectedIcon:       " ",
		LoadingHint:          "Sprinkling fairy dust...",
		SpinnerCharset:       []string{"✨", "🧚", "🌸", "✨"},
		SelectedPaddingRight: 6,
		MenuAlignment:        lipgloss.Center,
		HelpHint:             "✨ Dream in code",
	}
	// 13. Nord
	NordTheme = &Theme{
		Name:                 "Nord",
		TitleStyle:           lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#d8dee9")).Background(lipgloss.Color("#2e3440")).Align(lipgloss.Center),
		ItemStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#d8dee9")).Background(lipgloss.Color("#3b4252")).PaddingLeft(2),
		ItemStyleAlt:         lipgloss.NewStyle().Foreground(lipgloss.Color("#d8dee9")).Background(lipgloss.Color("#434c5e")).PaddingLeft(2),
		SelectedItem:         lipgloss.NewStyle().Foreground(lipgloss.Color("#3b4252")).Background(lipgloss.Color("#88c0d0")).Bold(true).Border(lipgloss.NormalBorder(), true).BorderForeground(lipgloss.Color("#88c0d0")).PaddingLeft(2).PaddingRight(4),
		FilterStyle:          lipgloss.NewStyle().Foreground(lipgloss.Color("#3b4252")).Background(lipgloss.Color("#88c0d0")).PaddingLeft(2).MarginTop(1).MarginBottom(1),
		HelpStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#d8dee9")).Background(lipgloss.Color("#2e3440")).MarginTop(1).Padding(0, 1),
		MainBg:               lipgloss.Color("#3b4252"),
		MainBorder:           lipgloss.Color("#88c0d0"),
		StatusBg:             lipgloss.Color("#2e3440"),
		StatusFg:             lipgloss.Color("#d8dee9"),
		TitleBg:              lipgloss.Color("#2e3440"),
		TitleFg:              lipgloss.Color("#d8dee9"),
		BorderStyle:          lipgloss.NormalBorder(),
		SelectionIcon:        "❄️",
		UnselectedIcon:       " ",
		LoadingHint:          "Crossing the fjord...",
		SpinnerCharset:       []string{"❄️", "🌨️", "💧", " "},
		SelectedPaddingRight: 4,
		MenuAlignment:        lipgloss.Center,
		HelpHint:             "Stay frosty",
	}
	// 14. Catppuccin
	CatppuccinTheme = &Theme{
		Name:                 "Catppuccin",
		TitleStyle:           lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f5e0dc")).Background(lipgloss.Color("#a6adc8")).Align(lipgloss.Center),
		ItemStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#f5e0dc")).Background(lipgloss.Color("#45475a")).PaddingLeft(2),
		ItemStyleAlt:         lipgloss.NewStyle().Foreground(lipgloss.Color("#f5e0dc")).Background(lipgloss.Color("#a6adc8")).PaddingLeft(2),
		SelectedItem:         lipgloss.NewStyle().Foreground(lipgloss.Color("#45475a")).Background(lipgloss.Color("#f5e0dc")).Bold(true).Border(lipgloss.RoundedBorder(), true).BorderForeground(lipgloss.Color("#f5e0dc")).PaddingLeft(2).PaddingRight(6),
		FilterStyle:          lipgloss.NewStyle().Foreground(lipgloss.Color("#45475a")).Background(lipgloss.Color("#f5e0dc")).PaddingLeft(2).MarginTop(1).MarginBottom(1),
		HelpStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#f5e0dc")).Background(lipgloss.Color("#a6adc8")).MarginTop(1).Padding(0, 1),
		MainBg:               lipgloss.Color("#45475a"),
		MainBorder:           lipgloss.Color("#a6adc8"),
		StatusBg:             lipgloss.Color("#a6adc8"),
		StatusFg:             lipgloss.Color("#f5e0dc"),
		TitleBg:              lipgloss.Color("#a6adc8"),
		TitleFg:              lipgloss.Color("#f5e0dc"),
		BorderStyle:          lipgloss.RoundedBorder(),
		SelectionIcon:        "🐾",
		UnselectedIcon:       " ",
		LoadingHint:          "Warming the milk...",
		SpinnerCharset:       []string{"🐱", "🐾", "☕", " "},
		SelectedPaddingRight: 6,
		MenuAlignment:        lipgloss.Center,
		HelpHint:             "Meow!",
	}
	// 15. Blackbird
	BlackbirdTheme = &Theme{
		Name:                 "Blackbird",
		TitleStyle:           lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f6c177")).Background(lipgloss.Color("#181818")).Align(lipgloss.Center),
		ItemStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#f6c177")).Background(lipgloss.Color("#181818")).PaddingLeft(2),
		ItemStyleAlt:         lipgloss.NewStyle().Foreground(lipgloss.Color("#f6c177")).Background(lipgloss.Color("#22223b")).PaddingLeft(2),
		SelectedItem:         lipgloss.NewStyle().Foreground(lipgloss.Color("#181818")).Background(lipgloss.Color("#f6c177")).Bold(true).Border(lipgloss.NormalBorder(), true).BorderForeground(lipgloss.Color("#f6c177")).PaddingLeft(2).PaddingRight(4),
		FilterStyle:          lipgloss.NewStyle().Foreground(lipgloss.Color("#181818")).Background(lipgloss.Color("#f6c177")).PaddingLeft(2).MarginTop(1).MarginBottom(1),
		HelpStyle:            lipgloss.NewStyle().Foreground(lipgloss.Color("#f6c177")).Background(lipgloss.Color("#181818")).MarginTop(1).Padding(0, 1),
		MainBg:               lipgloss.Color("#181818"),
		MainBorder:           lipgloss.Color("#f6c177"),
		StatusBg:             lipgloss.Color("#181818"),
		StatusFg:             lipgloss.Color("#f6c177"),
		TitleBg:              lipgloss.Color("#181818"),
		TitleFg:              lipgloss.Color("#f6c177"),
		BorderStyle:          lipgloss.NormalBorder(),
		SelectionIcon:        "🐦",
		UnselectedIcon:       " ",
		LoadingHint:          "Taking flight...",
		SpinnerCharset:       []string{"🐦", "⬛", "⬜", " "},
		SelectedPaddingRight: 4,
		MenuAlignment:        lipgloss.Center,
		HelpHint:             "Fly high",
	}
)

var (
	CurrentTheme = ModernPastelTheme
	allThemes    = []*Theme{
		SimpleCIDETheme, ModernPastelTheme, GhostyTheme, SolarizedDarkTheme, DraculaTheme, PacManTheme, MatrixTheme, GameboyTheme, DOSANSITheme, CyberpunkTheme, ZeldaTheme, MacClassicTheme, C64Theme,
		SynthwaveTheme, FairyflossTheme, NordTheme, CatppuccinTheme, BlackbirdTheme,
	}
)

func GetThemeNames() []string {
	names := make([]string, len(allThemes))
	for i, t := range allThemes {
		names[i] = t.Name
	}
	return names
}

func SetThemeByName(name string) {
	for _, t := range allThemes {
		if t.Name == name {
			CurrentTheme = t
			return
		}
	}
}

// Persist selected theme to ~/.ecs_cli_theme
func SaveThemeSelection(name string) {
	file := os.Getenv("HOME") + "/.ecs_cli_theme"
	_ = os.WriteFile(file, []byte(name), 0600)
}

// Load selected theme from ~/.ecs_cli_theme
func LoadThemeSelection() string {
	file := os.Getenv("HOME") + "/.ecs_cli_theme"
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
