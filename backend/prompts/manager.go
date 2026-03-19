package prompts

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"

	"gopkg.in/yaml.v3"
)

// Manager 提示词管理器
type Manager struct {
	basePath  string
	templates map[string]*Template
	mutex     sync.RWMutex
}

var (
	defaultManager *Manager
	managerOnce    sync.Once
)

// GetManager 获取默认管理器
func GetManager() *Manager {
	managerOnce.Do(func() {
		defaultManager = NewManager("./prompts")
	})
	return defaultManager
}

// NewManager 创建管理器
func NewManager(basePath string) *Manager {
	return &Manager{
		basePath:  basePath,
		templates: make(map[string]*Template),
	}
}

// Load 加载提示词模板
func (m *Manager) Load(name string) (*Template, error) {
	m.mutex.RLock()
	if tmpl, ok := m.templates[name]; ok {
		m.mutex.RUnlock()
		return tmpl, nil
	}
	m.mutex.RUnlock()

	// 从文件加载
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 双重检查
	if tmpl, ok := m.templates[name]; ok {
		return tmpl, nil
	}

	tmpl, err := m.loadFromFile(name)
	if err != nil {
		return nil, err
	}

	m.templates[name] = tmpl
	return tmpl, nil
}

// loadFromFile 从文件加载
func (m *Manager) loadFromFile(name string) (*Template, error) {
	// 尝试不同的扩展名
	exts := []string{".yaml", ".yml", ".txt"}
	var data []byte
	var found bool

	// 首先尝试直接加载
	for _, ext := range exts {
		path := filepath.Join(m.basePath, name+ext)
		data, _ = os.ReadFile(path)
		if data != nil {
			found = true
			break
		}
	}

	// 如果直接加载失败，尝试从子目录加载
	if !found {
		subdirs := []string{"tasks", "system"}
		for _, subdir := range subdirs {
			for _, ext := range exts {
				path := filepath.Join(m.basePath, subdir, name+ext)
				data, _ = os.ReadFile(path)
				if data != nil {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
	}

	if !found {
		return nil, fmt.Errorf("prompt template not found: %s", name)
	}

	// 解析YAML
	var tmpl Template
	if err := yaml.Unmarshal(data, &tmpl); err != nil {
		// 如果不是YAML，当作纯文本处理
		tmpl = Template{
			Name:    name,
			Version: "1.0.0",
			System:  "",
			User:    string(data),
		}
	}

	// 处理import
	if err := m.processImports(&tmpl); err != nil {
		return nil, err
	}

	return &tmpl, nil
}

// processImports 处理import语句
func (m *Manager) processImports(tmpl *Template) error {
	// 处理system中的import
	if strings.Contains(tmpl.System, "{{import") {
		system, err := m.replaceImports(tmpl.System)
		if err != nil {
			return err
		}
		tmpl.System = system
	}

	// 处理user中的import
	if strings.Contains(tmpl.User, "{{import") {
		user, err := m.replaceImports(tmpl.User)
		if err != nil {
			return err
		}
		tmpl.User = user
	}

	return nil
}

// replaceImports 替换import语句
func (m *Manager) replaceImports(content string) (string, error) {
	for {
		// 查找第一个import
		start := strings.Index(content, "{{import")
		if start == -1 {
			break
		}

		end := strings.Index(content[start:], "}}")
		if end == -1 {
			break
		}
		end += start + 2

		importStmt := content[start:end]

		// 提取文件路径 - 使用正则表达式匹配 {{import "path"}}
		re := regexp.MustCompile(`\{\{\s*import\s+"([^"]+)"\s*\}\}`)
		matches := re.FindStringSubmatch(importStmt)
		if len(matches) < 2 {
			return "", fmt.Errorf("invalid import statement: %s", importStmt)
		}
		importPath := matches[1]

		// 加载文件内容
		importContent, err := m.loadImportFile(importPath)
		if err != nil {
			return "", fmt.Errorf("failed to import %s: %v", importPath, err)
		}

		// 替换
		content = content[:start] + importContent + content[end:]
	}

	return content, nil
}

// loadImportFile 加载import文件
func (m *Manager) loadImportFile(importPath string) (string, error) {
	fullPath := filepath.Join(m.basePath, importPath)

	// 尝试添加扩展名
	if !strings.Contains(filepath.Base(fullPath), ".") {
		for _, ext := range []string{".txt", ".md", ".yaml"} {
			if data, err := os.ReadFile(fullPath + ext); err == nil {
				return string(data), nil
			}
		}
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// Template 提示词模板
type Template struct {
	Name        string                 `yaml:"name"`
	Version     string                 `yaml:"version"`
	Description string                 `yaml:"description"`
	Variables   []Variable             `yaml:"variables,omitempty"`
	System      string                 `yaml:"system,omitempty"`
	User        string                 `yaml:"user"`
	Examples    []Example              `yaml:"examples,omitempty"`
	OutputSchema map[string]interface{} `yaml:"output_schema,omitempty"`
}

// Variable 变量定义
type Variable struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required,omitempty"`
	Default     string `yaml:"default,omitempty"`
}

// Example 示例
type Example struct {
	Input  map[string]interface{} `yaml:"input"`
	Output string                 `yaml:"output"`
}

// Render 渲染模板
func (t *Template) Render(variables map[string]interface{}) (*RenderResult, error) {
	// 验证必需变量
	for _, v := range t.Variables {
		if v.Required {
			if _, ok := variables[v.Name]; !ok {
				return nil, fmt.Errorf("required variable missing: %s", v.Name)
			}
		}
		// 设置默认值
		if v.Default != "" {
			if _, ok := variables[v.Name]; !ok {
				variables[v.Name] = v.Default
			}
		}
	}

	// 渲染system
	system, err := t.renderTemplate(t.System, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to render system prompt: %v", err)
	}

	// 渲染user
	user, err := t.renderTemplate(t.User, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to render user prompt: %v", err)
	}

	return &RenderResult{
		System: system,
		User:   user,
		Full:   system + "\n\n" + user,
	}, nil
}

// renderTemplate 渲染单个模板
func (t *Template) renderTemplate(tmplStr string, variables map[string]interface{}) (string, error) {
	if tmplStr == "" {
		return "", nil
	}

	// 创建模板
	tmpl, err := template.New("prompt").Parse(tmplStr)
	if err != nil {
		return "", err
	}

	// 执行模板
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, variables); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// RenderResult 渲染结果
type RenderResult struct {
	System string `json:"system"`
	User   string `json:"user"`
	Full   string `json:"full"`
}

// GetMessages 转换为LLM消息格式
func (r *RenderResult) GetMessages() []map[string]string {
	messages := make([]map[string]string, 0)
	if r.System != "" {
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": r.System,
		})
	}
	if r.User != "" {
		messages = append(messages, map[string]string{
			"role":    "user",
			"content": r.User,
		})
	}
	return messages
}

// ListTemplates 列出所有模板
func (m *Manager) ListTemplates() ([]string, error) {
	var templates []string

	err := filepath.Walk(m.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		if ext == ".yaml" || ext == ".yml" || ext == ".txt" {
			relPath, _ := filepath.Rel(m.basePath, path)
			name := strings.TrimSuffix(relPath, ext)
			templates = append(templates, name)
		}

		return nil
	})

	return templates, err
}

// Reload 重新加载模板
func (m *Manager) Reload(name string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.templates, name)
	return nil
}

// ReloadAll 重新加载所有模板
func (m *Manager) ReloadAll() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.templates = make(map[string]*Template)
}
