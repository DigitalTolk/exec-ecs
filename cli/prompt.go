package cli

import (
	"context"
	"fmt"
	"log"
	"os"
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

const (
	itemsPerPage = 10
)

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
	items         []string
	filteredItems []string
	cursor        int
	choice        string
	label         string
	quitting      bool
	viewport      viewport.Model
	textInput     textinput.Model
	filterMode    bool
	page          int
}

func initialModel(label string, items []string) menuModel {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.CharLimit = 50
	ti.Width = 30

	return menuModel{
		items:         items,
		filteredItems: items,
		label:         label,
		viewport:      viewport.New(80, itemsPerPage+2),
		textInput:     ti,
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
				m.choice = m.filteredItems[m.cursor+m.page*itemsPerPage]
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
	help := "\n↑/↓: Navigate • /: Filter • Enter: Select • q: Quit"
	if m.filterMode {
		help = "\nEsc: Exit Filter • Enter: Apply Filter"
	}
	s.WriteString(helpStyle.Render(help))

	return s.String()
}

func bubbleteaSelect(label string, items []string) (string, error) {
	m := initialModel(label, items)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}

	mm, ok := finalModel.(menuModel)
	if !ok {
		return "", fmt.Errorf("unexpected model type")
	}
	return mm.choice, nil
}

func (c *Cli) PromptSelect(label string, items []string) string {
	selectedItem, err := bubbleteaSelect(label, items)
	if err != nil {
		log.Fatalf("Selection prompt failed: %v", err)
	}
	if selectedItem == "" {
		log.Fatalf("No selection made, exiting.")
	}
	return selectedItem
}

func (c *Cli) PromptWithDefault(label, defaultValue string, items []string) string {
	// allItems := append([]string{defaultValue}, items...)
	allItems := items
	selectedItem, err := bubbleteaSelect(label, allItems)
	if err != nil {
		log.Fatalf("Selection prompt failed: %v", err)
	}
	if selectedItem == "" {
		log.Fatalf("No selection made, exiting.")
	}
	return selectedItem
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
	selectedProfile := c.PromptSelect("Choose AWS profile", profiles)
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
	selectedClusterName := c.PromptSelect("Choose ECS cluster", clusterNames)

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
	selectedServiceName := c.PromptSelect("Choose ECS service", serviceNames)

	return selectedServiceName, nil
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
	selectedTask := c.PromptSelect("Choose ECS task", taskArns)
	return selectedTask, nil
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

	selectedContainer := c.PromptSelect("Choose a container", containerNames)
	return selectedContainer, nil
}
