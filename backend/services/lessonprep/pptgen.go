package lessonprep

import (
	"archive/zip"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hock1024always/GoEdu/models"
	"github.com/hock1024always/GoEdu/services/llm"
)

// PPTService PPT 生成服务
type PPTService struct {
	llmClient *llm.Client
	outputDir string
}

// NewPPTService 创建 PPT 服务
func NewPPTService(outputDir string) *PPTService {
	if outputDir == "" {
		outputDir = "runtime/ppt"
	}
	os.MkdirAll(outputDir, 0755)
	return &PPTService{
		llmClient: llm.NewClient(),
		outputDir: outputDir,
	}
}

// GeneratePPT 基于教案生成 PPT
func (s *PPTService) GeneratePPT(workflowID string, outline *models.CourseOutline, lessonPlan string) (string, error) {
	// 1. 调用 LLM 生成 PPT 结构
	pptStructure, err := s.generatePPTStructure(outline, lessonPlan)
	if err != nil {
		return "", fmt.Errorf("生成 PPT 结构失败: %v", err)
	}

	// 2. 生成 PPTX 文件
	fileName := fmt.Sprintf("%s_%s.pptx", workflowID[:8], sanitizeFileName(outline.Title))
	outputPath := filepath.Join(s.outputDir, fileName)

	if err := s.createPPTX(outputPath, pptStructure, outline); err != nil {
		return "", fmt.Errorf("创建 PPTX 文件失败: %v", err)
	}

	return outputPath, nil
}

// generatePPTStructure 调用 LLM 生成 PPT 页面结构
func (s *PPTService) generatePPTStructure(outline *models.CourseOutline, lessonPlan string) (*models.PPTStructure, error) {
	outlineJSON, _ := json.MarshalIndent(outline, "", "  ")

	// 截取教案内容（避免超过 token 限制）
	planContent := lessonPlan
	if len(planContent) > 4000 {
		planContent = planContent[:4000] + "\n...(内容截断)"
	}

	prompt := fmt.Sprintf(`基于以下课程大纲和教案，设计 PPT 的页面结构。

## 课程大纲
%s

## 教案内容
%s

请设计 12-18 页 PPT，严格按以下 JSON 格式输出：
{
    "slides": [
        {"type": "title", "title": "课程标题", "subtitle": "副标题（学科/年级/教师）"},
        {"type": "content", "title": "目录", "bullets": ["第一部分 xxx", "第二部分 xxx"]},
        {"type": "content", "title": "教学目标", "bullets": ["目标1", "目标2", "目标3"]},
        {"type": "content", "title": "章节标题", "bullets": ["要点1", "要点2", "要点3"], "notes": "演讲备注"},
        {"type": "interactive", "title": "课堂讨论", "bullets": ["讨论问题1", "讨论问题2"]},
        {"type": "summary", "title": "课堂总结", "bullets": ["总结要点1", "总结要点2"]}
    ]
}

设计原则：
1. 每页内容精炼，3-5 个要点
2. 包含导入、新授、练习、小结等教学环节
3. 至少 1 页互动/讨论页
4. 最后以总结+作业页结束
5. 只输出 JSON`, string(outlineJSON), planContent)

	resp, err := s.llmClient.Chat([]llm.Message{
		{Role: "system", Content: "你是一位 PPT 设计专家，擅长将教案内容转化为结构清晰、重点突出的演示文稿。只输出 JSON。"},
		{Role: "user", Content: prompt},
	}, nil)
	if err != nil {
		return nil, err
	}

	content := resp.Choices[0].Message.Content

	// 解析 JSON
	var pptStructure models.PPTStructure

	// 尝试多种方式解析
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &pptStructure); err != nil {
		jsonStr := extractJSONFromMarkdown(content)
		if jsonStr != "" {
			if err2 := json.Unmarshal([]byte(jsonStr), &pptStructure); err2 == nil {
				return &pptStructure, nil
			}
		}
		start := strings.Index(content, "{")
		end := strings.LastIndex(content, "}")
		if start >= 0 && end > start {
			if err2 := json.Unmarshal([]byte(content[start:end+1]), &pptStructure); err2 == nil {
				return &pptStructure, nil
			}
		}
		return nil, fmt.Errorf("解析 PPT 结构失败: %v", err)
	}

	return &pptStructure, nil
}

// createPPTX 创建 PPTX 文件
func (s *PPTService) createPPTX(outputPath string, structure *models.PPTStructure, outline *models.CourseOutline) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	slideCount := len(structure.Slides)

	// 写入 [Content_Types].xml
	if err := writeZipFile(w, "[Content_Types].xml", buildContentTypes(slideCount)); err != nil {
		return err
	}

	// 写入 _rels/.rels
	if err := writeZipFile(w, "_rels/.rels", relsXML); err != nil {
		return err
	}

	// 写入 docProps
	if err := writeZipFile(w, "docProps/app.xml", appXML); err != nil {
		return err
	}
	if err := writeZipFile(w, "docProps/core.xml", coreXML); err != nil {
		return err
	}

	// 写入 ppt/presentation.xml
	if err := writeZipFile(w, "ppt/presentation.xml", buildPresentationXML(slideCount)); err != nil {
		return err
	}

	// 写入 ppt/_rels/presentation.xml.rels
	if err := writeZipFile(w, "ppt/_rels/presentation.xml.rels", buildPresentationRels(slideCount)); err != nil {
		return err
	}

	// 写入主题
	if err := writeZipFile(w, "ppt/theme/theme1.xml", themeXML); err != nil {
		return err
	}

	// 写入 slideMaster
	if err := writeZipFile(w, "ppt/slideMasters/slideMaster1.xml", slideMasterXML); err != nil {
		return err
	}
	if err := writeZipFile(w, "ppt/slideMasters/_rels/slideMaster1.xml.rels", slideMasterRelsXML); err != nil {
		return err
	}

	// 写入 slideLayout
	if err := writeZipFile(w, "ppt/slideLayouts/slideLayout1.xml", slideLayoutXML); err != nil {
		return err
	}
	if err := writeZipFile(w, "ppt/slideLayouts/_rels/slideLayout1.xml.rels", slideLayoutRelsXML); err != nil {
		return err
	}

	// 写入每张幻灯片
	for i, slide := range structure.Slides {
		slideNum := i + 1
		slideXML := buildSlideXML(slide)
		if err := writeZipFile(w, fmt.Sprintf("ppt/slides/slide%d.xml", slideNum), slideXML); err != nil {
			return err
		}
		if err := writeZipFile(w, fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", slideNum), slideRelsXML); err != nil {
			return err
		}
	}

	return nil
}

// writeZipFile 写入 ZIP 文件条目
func writeZipFile(w *zip.Writer, name, content string) error {
	f, err := w.Create(name)
	if err != nil {
		return err
	}
	_, err = f.Write([]byte(content))
	return err
}

// sanitizeFileName 清理文件名
func sanitizeFileName(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, " ", "_")
	if len(name) > 50 {
		name = name[:50]
	}
	return name
}

// xmlEscape 转义 XML 特殊字符
func xmlEscape(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

// buildSlideXML 根据幻灯片数据生成 slide XML
func buildSlideXML(slide models.PPTSlideData) string {
	switch slide.Type {
	case "title":
		return buildTitleSlide(slide)
	case "summary":
		return buildSummarySlide(slide)
	case "interactive":
		return buildInteractiveSlide(slide)
	default:
		return buildContentSlide(slide)
	}
}

// --- 各类型幻灯片 XML 生成 ---

func buildTitleSlide(slide models.PPTSlideData) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
       xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:cSld>
    <p:bg>
      <p:bgPr>
        <a:solidFill><a:srgbClr val="8B0000"/></a:solidFill>
        <a:effectLst/>
      </p:bgPr>
    </p:bg>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
      <p:grpSpPr/>
      <p:sp>
        <p:nvSpPr><p:cNvPr id="2" name="Title"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
        <p:spPr>
          <a:xfrm><a:off x="457200" y="2286000"/><a:ext cx="8229600" cy="1143000"/></a:xfrm>
          <a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
        </p:spPr>
        <p:txBody>
          <a:bodyPr anchor="ctr"/>
          <a:p>
            <a:pPr algn="ctr"/>
            <a:r>
              <a:rPr lang="zh-CN" sz="4400" b="1" dirty="0">
                <a:solidFill><a:srgbClr val="FFD700"/></a:solidFill>
                <a:latin typeface="Microsoft YaHei"/>
                <a:ea typeface="Microsoft YaHei"/>
              </a:rPr>
              <a:t>%s</a:t>
            </a:r>
          </a:p>
        </p:txBody>
      </p:sp>
      <p:sp>
        <p:nvSpPr><p:cNvPr id="3" name="Subtitle"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
        <p:spPr>
          <a:xfrm><a:off x="457200" y="3657600"/><a:ext cx="8229600" cy="685800"/></a:xfrm>
          <a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
        </p:spPr>
        <p:txBody>
          <a:bodyPr anchor="ctr"/>
          <a:p>
            <a:pPr algn="ctr"/>
            <a:r>
              <a:rPr lang="zh-CN" sz="2000" dirty="0">
                <a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill>
                <a:latin typeface="Microsoft YaHei"/>
                <a:ea typeface="Microsoft YaHei"/>
              </a:rPr>
              <a:t>%s</a:t>
            </a:r>
          </a:p>
        </p:txBody>
      </p:sp>
    </p:spTree>
  </p:cSld>
</p:sld>`, xmlEscape(slide.Title), xmlEscape(slide.Subtitle))
}

func buildContentSlide(slide models.PPTSlideData) string {
	var bullets strings.Builder
	for _, b := range slide.Bullets {
		bullets.WriteString(fmt.Sprintf(`
          <a:p>
            <a:pPr marL="342900" indent="-342900">
              <a:buChar char="%s"/>
            </a:pPr>
            <a:r>
              <a:rPr lang="zh-CN" sz="2000" dirty="0">
                <a:solidFill><a:srgbClr val="333333"/></a:solidFill>
                <a:latin typeface="Microsoft YaHei"/>
                <a:ea typeface="Microsoft YaHei"/>
              </a:rPr>
              <a:t>%s</a:t>
            </a:r>
          </a:p>`, "\u25cf", xmlEscape(b)))
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
       xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
      <p:grpSpPr/>
      <p:sp>
        <p:nvSpPr><p:cNvPr id="2" name="Header"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
        <p:spPr>
          <a:xfrm><a:off x="0" y="0"/><a:ext cx="9144000" cy="914400"/></a:xfrm>
          <a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
          <a:solidFill><a:srgbClr val="8B0000"/></a:solidFill>
        </p:spPr>
        <p:txBody>
          <a:bodyPr anchor="ctr"/>
          <a:p>
            <a:pPr algn="l"/>
            <a:r>
              <a:rPr lang="zh-CN" sz="2800" b="1" dirty="0">
                <a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill>
                <a:latin typeface="Microsoft YaHei"/>
                <a:ea typeface="Microsoft YaHei"/>
              </a:rPr>
              <a:t>  %s</a:t>
            </a:r>
          </a:p>
        </p:txBody>
      </p:sp>
      <p:sp>
        <p:nvSpPr><p:cNvPr id="3" name="Content"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
        <p:spPr>
          <a:xfrm><a:off x="457200" y="1143000"/><a:ext cx="8229600" cy="4800600"/></a:xfrm>
          <a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
        </p:spPr>
        <p:txBody>
          <a:bodyPr wrap="square"/>%s
        </p:txBody>
      </p:sp>
    </p:spTree>
  </p:cSld>
</p:sld>`, xmlEscape(slide.Title), bullets.String())
}

func buildInteractiveSlide(slide models.PPTSlideData) string {
	var bullets strings.Builder
	for _, b := range slide.Bullets {
		bullets.WriteString(fmt.Sprintf(`
          <a:p>
            <a:pPr marL="342900" indent="-342900">
              <a:buChar char="%s"/>
            </a:pPr>
            <a:r>
              <a:rPr lang="zh-CN" sz="2200" dirty="0">
                <a:solidFill><a:srgbClr val="1A1A1A"/></a:solidFill>
                <a:latin typeface="Microsoft YaHei"/>
                <a:ea typeface="Microsoft YaHei"/>
              </a:rPr>
              <a:t>%s</a:t>
            </a:r>
          </a:p>`, "\u2753", xmlEscape(b)))
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
       xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:cSld>
    <p:bg>
      <p:bgPr>
        <a:solidFill><a:srgbClr val="FFF8DC"/></a:solidFill>
        <a:effectLst/>
      </p:bgPr>
    </p:bg>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
      <p:grpSpPr/>
      <p:sp>
        <p:nvSpPr><p:cNvPr id="2" name="Title"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
        <p:spPr>
          <a:xfrm><a:off x="457200" y="457200"/><a:ext cx="8229600" cy="914400"/></a:xfrm>
          <a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
        </p:spPr>
        <p:txBody>
          <a:bodyPr anchor="ctr"/>
          <a:p>
            <a:pPr algn="ctr"/>
            <a:r>
              <a:rPr lang="zh-CN" sz="3200" b="1" dirty="0">
                <a:solidFill><a:srgbClr val="8B0000"/></a:solidFill>
                <a:latin typeface="Microsoft YaHei"/>
                <a:ea typeface="Microsoft YaHei"/>
              </a:rPr>
              <a:t>%s</a:t>
            </a:r>
          </a:p>
        </p:txBody>
      </p:sp>
      <p:sp>
        <p:nvSpPr><p:cNvPr id="3" name="Content"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
        <p:spPr>
          <a:xfrm><a:off x="457200" y="1600200"/><a:ext cx="8229600" cy="4114800"/></a:xfrm>
          <a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
        </p:spPr>
        <p:txBody>
          <a:bodyPr wrap="square"/>%s
        </p:txBody>
      </p:sp>
    </p:spTree>
  </p:cSld>
</p:sld>`, xmlEscape(slide.Title), bullets.String())
}

func buildSummarySlide(slide models.PPTSlideData) string {
	var bullets strings.Builder
	for _, b := range slide.Bullets {
		bullets.WriteString(fmt.Sprintf(`
          <a:p>
            <a:pPr marL="342900" indent="-342900">
              <a:buChar char="%s"/>
            </a:pPr>
            <a:r>
              <a:rPr lang="zh-CN" sz="2200" dirty="0">
                <a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill>
                <a:latin typeface="Microsoft YaHei"/>
                <a:ea typeface="Microsoft YaHei"/>
              </a:rPr>
              <a:t>%s</a:t>
            </a:r>
          </a:p>`, "\u2605", xmlEscape(b)))
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
       xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:cSld>
    <p:bg>
      <p:bgPr>
        <a:solidFill><a:srgbClr val="8B0000"/></a:solidFill>
        <a:effectLst/>
      </p:bgPr>
    </p:bg>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
      <p:grpSpPr/>
      <p:sp>
        <p:nvSpPr><p:cNvPr id="2" name="Title"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
        <p:spPr>
          <a:xfrm><a:off x="457200" y="457200"/><a:ext cx="8229600" cy="914400"/></a:xfrm>
          <a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
        </p:spPr>
        <p:txBody>
          <a:bodyPr anchor="ctr"/>
          <a:p>
            <a:pPr algn="ctr"/>
            <a:r>
              <a:rPr lang="zh-CN" sz="3600" b="1" dirty="0">
                <a:solidFill><a:srgbClr val="FFD700"/></a:solidFill>
                <a:latin typeface="Microsoft YaHei"/>
                <a:ea typeface="Microsoft YaHei"/>
              </a:rPr>
              <a:t>%s</a:t>
            </a:r>
          </a:p>
        </p:txBody>
      </p:sp>
      <p:sp>
        <p:nvSpPr><p:cNvPr id="3" name="Content"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
        <p:spPr>
          <a:xfrm><a:off x="457200" y="1600200"/><a:ext cx="8229600" cy="4114800"/></a:xfrm>
          <a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
        </p:spPr>
        <p:txBody>
          <a:bodyPr wrap="square"/>%s
        </p:txBody>
      </p:sp>
    </p:spTree>
  </p:cSld>
</p:sld>`, xmlEscape(slide.Title), bullets.String())
}

// --- OOXML 模板常量 ---

func buildContentTypes(slideCount int) string {
	var slides strings.Builder
	for i := 1; i <= slideCount; i++ {
		slides.WriteString(fmt.Sprintf(`
  <Override PartName="/ppt/slides/slide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`, i))
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="xml" ContentType="application/xml"/>
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>
  <Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>
  <Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>
  <Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>
  <Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>%s
</Types>`, slides.String())
}

func buildPresentationXML(slideCount int) string {
	var slideList strings.Builder
	for i := 1; i <= slideCount; i++ {
		slideList.WriteString(fmt.Sprintf(`
      <p:sldId id="%d" r:id="rId%d"/>`, 255+i, 10+i))
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
                xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
                xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:sldMasterIdLst>
    <p:sldMasterId id="2147483648" r:id="rId1"/>
  </p:sldMasterIdLst>
  <p:sldIdLst>%s
  </p:sldIdLst>
  <p:sldSz cx="9144000" cy="6858000" type="screen4x3"/>
  <p:notesSz cx="6858000" cy="9144000"/>
</p:presentation>`, slideList.String())
}

func buildPresentationRels(slideCount int) string {
	var slideRels strings.Builder
	for i := 1; i <= slideCount; i++ {
		slideRels.WriteString(fmt.Sprintf(`
  <Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide%d.xml"/>`, 10+i, i))
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>%s
</Relationships>`, slideRels.String())
}

const relsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
</Relationships>`

const appXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties">
  <Application>AI Education System</Application>
  <Company>AIEducation</Company>
</Properties>`

const coreXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties"
                   xmlns:dc="http://purl.org/dc/elements/1.1/">
  <dc:title>AI Generated Lesson PPT</dc:title>
  <dc:creator>AI Education System</dc:creator>
</cp:coreProperties>`

const slideRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
</Relationships>`

const slideMasterXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
             xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
             xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:cSld>
    <p:bg>
      <p:bgPr>
        <a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill>
        <a:effectLst/>
      </p:bgPr>
    </p:bg>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
      <p:grpSpPr/>
    </p:spTree>
  </p:cSld>
  <p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2"
            accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>
  <p:sldLayoutIdLst>
    <p:sldLayoutId id="2147483649" r:id="rId1"/>
  </p:sldLayoutIdLst>
</p:sldMaster>`

const slideMasterRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>
</Relationships>`

const slideLayoutXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
             xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
             xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
             type="blank">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
      <p:grpSpPr/>
    </p:spTree>
  </p:cSld>
</p:sldLayout>`

const slideLayoutRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/>
</Relationships>`

const themeXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="HistoryTheme">
  <a:themeElements>
    <a:clrScheme name="History">
      <a:dk1><a:srgbClr val="1A1A1A"/></a:dk1>
      <a:lt1><a:srgbClr val="FFFFFF"/></a:lt1>
      <a:dk2><a:srgbClr val="333333"/></a:dk2>
      <a:lt2><a:srgbClr val="F5F5DC"/></a:lt2>
      <a:accent1><a:srgbClr val="8B0000"/></a:accent1>
      <a:accent2><a:srgbClr val="FFD700"/></a:accent2>
      <a:accent3><a:srgbClr val="006400"/></a:accent3>
      <a:accent4><a:srgbClr val="00008B"/></a:accent4>
      <a:accent5><a:srgbClr val="8B4513"/></a:accent5>
      <a:accent6><a:srgbClr val="4B0082"/></a:accent6>
      <a:hlink><a:srgbClr val="0000FF"/></a:hlink>
      <a:folHlink><a:srgbClr val="800080"/></a:folHlink>
    </a:clrScheme>
    <a:fontScheme name="History">
      <a:majorFont>
        <a:latin typeface="Microsoft YaHei"/>
        <a:ea typeface="Microsoft YaHei"/>
        <a:cs typeface=""/>
      </a:majorFont>
      <a:minorFont>
        <a:latin typeface="Microsoft YaHei"/>
        <a:ea typeface="Microsoft YaHei"/>
        <a:cs typeface=""/>
      </a:minorFont>
    </a:fontScheme>
    <a:fmtScheme name="Office">
      <a:fillStyleLst>
        <a:solidFill><a:schemeClr val="phClr"/></a:solidFill>
        <a:solidFill><a:schemeClr val="phClr"/></a:solidFill>
        <a:solidFill><a:schemeClr val="phClr"/></a:solidFill>
      </a:fillStyleLst>
      <a:lnStyleLst>
        <a:ln w="9525"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln>
        <a:ln w="25400"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln>
        <a:ln w="38100"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln>
      </a:lnStyleLst>
      <a:effectStyleLst>
        <a:effectStyle><a:effectLst/></a:effectStyle>
        <a:effectStyle><a:effectLst/></a:effectStyle>
        <a:effectStyle><a:effectLst/></a:effectStyle>
      </a:effectStyleLst>
      <a:bgFillStyleLst>
        <a:solidFill><a:schemeClr val="phClr"/></a:solidFill>
        <a:solidFill><a:schemeClr val="phClr"/></a:solidFill>
        <a:solidFill><a:schemeClr val="phClr"/></a:solidFill>
      </a:bgFillStyleLst>
    </a:fmtScheme>
  </a:themeElements>
</a:theme>`
