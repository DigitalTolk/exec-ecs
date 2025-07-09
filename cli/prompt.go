package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/ini.v1"
)

// Add a constant for the Go Back option
const goBackOption = "← Go Back"

const (
	itemsPerPage = 10
)

const clearHistoryOption = "🗑️  Clear History"

var (
	// Styling
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)

	itemStyle = lipgloss.NewStyle().
			PaddingLeft(4)

	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("205")).
				Bold(true)

	filterStyle = lipgloss.NewStyle().
			PaddingLeft(4).
			MarginTop(1).
			MarginBottom(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)
)

type menuModel struct {
	items           []string
	filteredItems   []string
	cursor          int
	choice          string
	label           string
	quitting        bool
	viewport        viewport.Model
	textInput       textinput.Model
	filterMode      bool
	page            int
	historyMode     bool
	goBackTriggered bool
}

func initialModel(label string, items []string, defaultSelected string, showGoBack bool) menuModel {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.CharLimit = 50
	ti.Width = 30

	// Do NOT insert Go Back option in the list anymore
	allItems := items

	// Find the index of the default selected value
	selectedIdx := 0
	if defaultSelected != "" {
		for i, item := range allItems {
			if item == defaultSelected {
				selectedIdx = i
				break
			}
		}
	}
	page := selectedIdx / itemsPerPage
	cursor := selectedIdx % itemsPerPage

	return menuModel{
		items:         allItems,
		filteredItems: allItems,
		label:         label,
		viewport:      viewport.New(80, itemsPerPage+2),
		textInput:     ti,
		cursor:        cursor,
		page:          page,
	}
}

func (m menuModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *menuModel) filterItems(filter string) {
	if filter == "" {
		m.filteredItems = m.items
		return
	}

	filtered := make([]string, 0)
	for _, item := range m.items {
		if strings.Contains(strings.ToLower(item), strings.ToLower(filter)) {
			filtered = append(filtered, item)
		}
	}
	m.filteredItems = filtered
	m.cursor = 0
	m.page = 0
}

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "/":
			if !m.filterMode {
				m.filterMode = true
				m.textInput.Focus()
				return m, textinput.Blink
			}

		case "esc":
			if m.filterMode {
				m.filterMode = false
				m.textInput.Blur()
				return m, nil
			}
			m.quitting = true
			return m, tea.Quit

		case "enter":
			if m.filterMode {
				m.filterMode = false
				m.textInput.Blur()
				return m, nil
			}
			if len(m.filteredItems) > 0 {
				choice := m.filteredItems[m.cursor+m.page*itemsPerPage]
				m.choice = choice
				m.quitting = true
				return m, tea.Quit
			}

		case "up":
			if !m.filterMode {
				if m.cursor > 0 {
					m.cursor--
				} else if m.page > 0 {
					m.page--
					m.cursor = itemsPerPage - 1
				}
			}

		case "down":
			if !m.filterMode {
				maxIndex := min(itemsPerPage, len(m.filteredItems)-m.page*itemsPerPage)
				if m.cursor < maxIndex-1 {
					m.cursor++
				} else if (m.page+1)*itemsPerPage < len(m.filteredItems) {
					m.page++
					m.cursor = 0
				}
			}

		case "pgup":
			if !m.filterMode && m.page > 0 {
				m.page--
				m.cursor = 0
			}

		case "pgdown":
			if !m.filterMode && (m.page+1)*itemsPerPage < len(m.filteredItems) {
				m.page++
				m.cursor = 0
			}

		// Add ctrl+h for history
		case "ctrl+h":
			// Show history menu
			history := GetLastUniqueHistory(5)
			if len(history) == 0 {
				return m, nil
			}
			selected, err := BubbleteaHistorySelect("Command History (last 5 unique)", history)
			if err == nil && selected != "" {
				// Execute the selected command
				fmt.Println("\nExecuting:", selected)
				cmd := exec.Command("sh", "-c", selected)
				cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
				cmd.Stdin = os.Stdin
				_ = cmd.Run()
			}
			return m, nil

		// Add ctrl+left for go back
		case "ctrl+left":
			m.goBackTriggered = true
			m.quitting = true
			return m, tea.Quit

		// Add 'ctrl+b' for go back
		case "ctrl+b":
			m.goBackTriggered = true
			m.quitting = true
			return m, tea.Quit
		}
	}

	if m.filterMode {
		m.textInput, cmd = m.textInput.Update(msg)
		m.filterItems(m.textInput.Value())
		return m, cmd
	}

	return m, nil
}

func (m menuModel) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder

	// Title
	s.WriteString(titleStyle.Render(m.label))

	// Filter input
	if m.filterMode {
		s.WriteString(filterStyle.Render("Filter: " + m.textInput.View()))
	}

	// Items
	start := m.page * itemsPerPage
	end := min(start+itemsPerPage, len(m.filteredItems))
	s.WriteString("\n")
	for i := start; i < end; i++ {
		item := m.filteredItems[i]
		if i-start == m.cursor {
			s.WriteString(selectedItemStyle.Render("➜ " + item))
		} else {
			s.WriteString(itemStyle.Render(item))
		}
		s.WriteString("\n")
	}

	// Pagination info
	if len(m.filteredItems) > itemsPerPage {
		s.WriteString(fmt.Sprintf("\nPage %d/%d", m.page+1, (len(m.filteredItems)-1)/itemsPerPage+1))
	}

	// Help
	if m.historyMode {
		help := "\nTo go back press esc key"
		s.WriteString(helpStyle.Render(help))
		return s.String()
	}

	help := "\n↑/↓: Navigate • /: Filter • Enter: Select • q: Quit"
	help2 := "\nctrl+b: Go Back • ctrl+h: History"
	if m.filterMode {
		help = "\nEsc: Exit Filter • Enter: Apply Filter"
		help2 = ""
	}
	s.WriteString(helpStyle.Render(help))
	if help2 != "" {
		s.WriteString(helpStyle.Render(help2))
	}

	return s.String()
}

func bubbleteaSelect(label string, items []string, defaultSelected string, showGoBack bool) (string, bool, error) {
	m := initialModel(label, items, defaultSelected, showGoBack)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return "", false, err
	}

	mm, ok := finalModel.(menuModel)
	if !ok {
		return "", false, fmt.Errorf("unexpected model type")
	}
	return mm.choice, mm.goBackTriggered, nil
}

func BubbleteaHistorySelect(label string, items []string) (string, error) {
	// Format each history item as multiline for display
	displayItems := make([]string, len(items))
	for i, cmd := range items {
		// Insert a newline after every 80 characters for readability
		var formatted strings.Builder
		count := 0
		for _, r := range cmd {
			formatted.WriteRune(r)
			count++
			if count >= 80 && r == ' ' {
				formatted.WriteRune('\n')
				count = 0
			}
		}
		displayItems[i] = formatted.String()
	}
	// Add 'Clear History' option at the bottom
	allItems := append(displayItems, clearHistoryOption)
	m := initialModel(label, allItems, "", false)
	m.historyMode = true
	for {
		p := tea.NewProgram(m)
		finalModel, err := p.Run()
		if err != nil {
			return "", err
		}
		mm, ok := finalModel.(menuModel)
		if !ok {
			return "", fmt.Errorf("unexpected model type")
		}
		if mm.choice == clearHistoryOption {
			// Clear the history file
			historyFile := os.Getenv("HOME") + "/.ecs_cli_history"
			_ = os.Remove(historyFile)
			// Show a message and re-show the menu (now empty)
			fmt.Println("History cleared.")
			// Remove all items except clear option
			allItems = []string{clearHistoryOption}
			m = initialModel(label, allItems, "", false)
			m.historyMode = true
			continue
		}
		// Map the selected display string back to the original command
		if mm.choice == clearHistoryOption {
			return mm.choice, nil
		}
		for i, disp := range displayItems {
			if mm.choice == disp {
				return items[i], nil
			}
		}
		return mm.choice, nil // fallback
	}
}

func (c *Cli) PromptWithDefault(label, defaultValue string, items []string, showGoBack bool) (string, bool) {
	allItems := items
	return c.PromptSelect(label, allItems, defaultValue, showGoBack)
}

// Helper function since Go 1.21's min() is not yet widely available
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

/* -------------------------------------------------------------------------
   4. The rest of your AWS-related methods
   ------------------------------------------------------------------------- */

func (c *Cli) CheckSSOSession(ctx context.Context, client *sts.Client, profile string) error {
	_, err := client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	return err
}

func (c *Cli) getStoredConfigPath() string {
	customPathFile := os.Getenv("HOME") + "/.aws/custom_config_path"
	if data, err := os.ReadFile(customPathFile); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

func (c *Cli) saveCustomConfigPath(path string) error {
	customPathFile := os.Getenv("HOME") + "/.aws/custom_config_path"
	awsDir := filepath.Dir(customPathFile)
	if err := os.MkdirAll(awsDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return os.WriteFile(customPathFile, []byte(path), 0600)
}

func (c *Cli) SelectProfile() string {
	awsConfigPath := c.getStoredConfigPath()
	if awsConfigPath == "" {
		awsConfigPath = os.Getenv("HOME") + "/.aws/config"
	}

	// If config doesn't exist, ask user to provide the path
	if _, err := os.Stat(awsConfigPath); os.IsNotExist(err) {
		fmt.Printf("AWS config file not found at %s.\n", awsConfigPath)
		fmt.Print("Please enter AWS config file path:\n> ")
		var newPath string
		_, scanErr := fmt.Scanln(&newPath)
		if scanErr != nil {
			log.Fatalf("Unable to read config path: %v", scanErr)
		}
		awsConfigPath = newPath

		if err := c.saveCustomConfigPath(newPath); err != nil {
			fmt.Printf("Warning: Failed to save custom config path: %v\n", err)
		}
	}

	cfg, err := ini.Load(awsConfigPath)
	if err != nil {
		log.Fatalf("Failed to load AWS config at %s: %v", awsConfigPath, err)
	}

	var profiles []string
	for _, section := range cfg.Sections() {
		if strings.HasPrefix(section.Name(), "profile ") {
			profiles = append(profiles, strings.TrimPrefix(section.Name(), "profile "))
		}
	}
	if len(profiles) == 0 {
		log.Fatalf("No profiles found in AWS config: %s", awsConfigPath)
	}

	// Let user choose from these profiles via Bubble Tea
	selectedProfile, _ := c.PromptSelect("Choose AWS profile", profiles, "", false)
	return selectedProfile
}

func (c *Cli) SelectCluster(ctx context.Context, client *ecs.Client) (string, error) {
	output, err := client.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		return "", err
	}
	clusters := output.ClusterArns
	if len(clusters) == 0 {
		return "", fmt.Errorf("no ECS clusters found")
	}

	// Extract just the final part of the ARN (the cluster name).
	clusterNames := make([]string, len(clusters))
	for i, arn := range clusters {
		parts := strings.Split(arn, "/")
		clusterNames[i] = parts[len(parts)-1]
	}

	// Let the user pick from the simplified cluster names.
	selectedClusterName, _ := c.PromptSelect("Choose ECS cluster", clusterNames, "", false)

	return selectedClusterName, nil
}

func (c *Cli) SelectService(ctx context.Context, client *ecs.Client, clusterArn string) (string, error) {
	maxResults := int32(100)
	output, err := client.ListServices(ctx, &ecs.ListServicesInput{
		Cluster:    &clusterArn,
		MaxResults: &maxResults,
	})
	if err != nil {
		return "", err
	}
	services := output.ServiceArns
	if len(services) == 0 {
		return "", fmt.Errorf("no services found in ECS cluster %s", clusterArn)
	}

	// Convert the full ARNs to just the service names
	serviceNames := make([]string, len(services))
	for i, arn := range services {
		parts := strings.Split(arn, "/")
		serviceNames[i] = parts[len(parts)-1]
	}

	// Show only the service names in the prompt
	selectedServiceName, _ := c.PromptSelect("Choose ECS service", serviceNames, "", false)

	return selectedServiceName, nil
}

func maskTaskArn(taskArn string) string {
	// If the ARN is too short to mask, return it as is
	if len(taskArn) <= 13 {
		return taskArn
	}
	// Keep the first and last 5 characters, mask the rest
	return taskArn[:3] + strings.Repeat("*", len(taskArn)-13) + taskArn[len(taskArn)-10:]
}

func (c *Cli) SelectTask(ctx context.Context, client *ecs.Client, clusterArn, serviceName string) (string, error) {
	var (
		taskArns  []string
		nextToken *string
	)

	for {
		output, err := client.ListTasks(ctx, &ecs.ListTasksInput{
			Cluster:     &clusterArn,
			ServiceName: &serviceName,
			NextToken:   nextToken,
		})
		if err != nil {
			return "", err
		}

		taskArns = append(taskArns, output.TaskArns...)
		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	if len(taskArns) == 0 {
		return "", fmt.Errorf("no tasks found for service %s", serviceName)
	}

	// Mask each task ARN before presenting them to the user
	maskedTaskArns := make([]string, len(taskArns))
	for i, arn := range taskArns {
		maskedTaskArns[i] = maskTaskArn(arn)
	}

	selectedMaskedTask, _ := c.PromptSelect("Choose ECS task", maskedTaskArns, "", false)

	// Find the original task ARN corresponding to the masked selection
	for i, maskedArn := range maskedTaskArns {
		if maskedArn == selectedMaskedTask {
			return taskArns[i], nil
		}
	}

	return "", fmt.Errorf("selected task not found")
}

func (c *Cli) SelectContainer(ctx context.Context, client *ecs.Client, clusterArn, taskArn string) (string, error) {
	output, err := client.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: &clusterArn,
		Tasks:   []string{taskArn},
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe tasks: %w", err)
	}
	if len(output.Tasks) == 0 {
		return "", fmt.Errorf("no tasks found for ARN %s", taskArn)
	}

	task := output.Tasks[0]
	if len(task.Containers) == 0 {
		return "", fmt.Errorf("no containers found in task %s", taskArn)
	}

	containerNames := make([]string, 0, len(task.Containers))
	for _, cont := range task.Containers {
		if cont.Name != nil {
			containerNames = append(containerNames, *cont.Name)
		}
	}

	selectedContainer, _ := c.PromptSelect("Choose a container", containerNames, "", false)
	return selectedContainer, nil
}
