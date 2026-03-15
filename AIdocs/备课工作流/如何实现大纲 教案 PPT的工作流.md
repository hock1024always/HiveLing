# AI 备课系统：大纲-教案-PPT 自动生成工作流

## 一、基础概念科普

### 1.1 什么是 AI 备课工作流？

**AI 备课工作流**是指利用大语言模型（LLM）和自动化工具，将传统教师备课的繁琐流程（查资料、写大纲、编教案、做课件）转化为**自动化流水线**，实现从课程主题到完整课件的一键生成。

### 1.2 传统备课 vs AI 备课

| 环节 | 传统备课 | AI 备课 |
|------|----------|---------|
| **大纲设计** | 2-3小时查阅资料 | 2-3分钟自动生成 |
| **教案编写** | 4-6小时手写 | 5-10分钟智能生成 |
| **PPT制作** | 3-4小时排版设计 | 3-5分钟自动排版 |
| **总计** | 8-12小时 | 10-20分钟 |

### 1.3 核心技术栈

```
┌─────────────────────────────────────────────────────────────┐
│                    AI 备课技术栈                             │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   大语言模型   │  │   文档生成    │  │   模板引擎    │      │
│  │   (LLM)      │  │   (python-   │  │   (Jinja2)   │      │
│  │              │  │    pptx)     │  │              │      │
│  │  GPT-4/     │  │              │  │  动态内容     │      │
│  │  Claude/    │  │  生成.pptx   │  │  填充模板     │      │
│  │  Qwen       │  │  格式文件    │  │              │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
│                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   知识检索    │  │   工作流引擎  │  │   内容审核    │      │
│  │   (RAG)      │  │   (LangChain)│  │   (Guardrails)│      │
│  │              │  │              │  │              │      │
│  │  向量数据库   │  │  任务编排    │  │  内容安全     │      │
│  │  本地知识库   │  │  状态管理    │  │  质量检查     │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## 二、系统架构设计

### 2.1 总体架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                         AI 备课系统架构                              │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                      用户交互层                              │   │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────────────┐    │   │
│  │  │  Web界面   │  │  输入表单   │  │  课程主题/年级/学科  │    │   │
│  │  └────────────┘  └────────────┘  └────────────────────┘    │   │
│  └────────────────────────┬────────────────────────────────────┘   │
│                           │                                         │
│                           ▼                                         │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                      工作流编排层                            │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐    │   │
│  │  │ 大纲生成  │──►│ 教案生成  │──►│ PPT生成  │──►│ 审核导出  │    │   │
│  │  │  Agent   │  │  Agent   │  │  Agent   │  │  Agent   │    │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘    │   │
│  └────────────────────────┬────────────────────────────────────┘   │
│                           │                                         │
│                           ▼                                         │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                      能力支撑层                              │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐    │   │
│  │  │  LLM API │  │  RAG检索  │  │ 模板管理  │  │ 资源库   │    │   │
│  │  │          │  │          │  │          │  │          │    │   │
│  │  │ GPT-4   │  │ 向量检索  │  │ PPT模板  │  │ 图片/    │    │   │
│  │  │ Claude  │  │ 知识图谱  │  │ 教案模板  │  │ 视频素材 │    │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘    │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 数据流设计

```
用户输入
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ Step 1: 意图理解与参数提取                                    │
│ 输入："帮我准备一节初中生物的光合作用课程"                      │
│ 输出：{学科: 生物, 年级: 初中, 主题: 光合作用}                  │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ Step 2: 大纲生成 (Outline Generation)                        │
│ - 调用 LLM 生成课程大纲                                       │
│ - 包含：教学目标、重点难点、课时安排、教学环节                   │
│ 输出：结构化大纲 JSON                                         │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ Step 3: 教案生成 (Lesson Plan Generation)                    │
│ - 基于大纲展开详细教案                                         │
│ - 包含：导入、新授、练习、小结、作业                           │
│ - 检索相关教学资源补充                                         │
│ 输出：完整教案文档                                            │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ Step 4: PPT 生成 (Presentation Generation)                   │
│ - 基于教案内容生成 PPT 结构                                    │
│ - 选择合适模板                                                │
│ - 填充内容并排版                                              │
│ 输出：.pptx 文件                                             │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ Step 5: 质量审核与导出                                        │
│ - 内容准确性检查                                              │
│ - 格式规范性检查                                              │
│ - 生成下载链接                                                │
└─────────────────────────────────────────────────────────────┘
```

---

## 三、详细流程设计

### 3.1 大纲生成流程

```python
# 大纲生成 Prompt 设计
OUTLINE_GENERATION_PROMPT = """
你是一位经验丰富的{subject}教师，请为{grade}学生设计一节关于"{topic}"的课程大纲。

要求：
1. 符合{grade}学生的认知水平
2. 课时：{duration}分钟
3. 包含以下模块：
   - 教学目标（知识、能力、情感三维目标）
   - 教学重难点
   - 教学方法
   - 教学过程（导入、新授、练习、小结、作业）
   - 板书设计

请以 JSON 格式输出：
{{
    "title": "课程标题",
    "duration": 课时数,
    "objectives": {{
        "knowledge": "知识目标",
        "ability": "能力目标",
        "emotion": "情感目标"
    }},
    "key_points": ["重点1", "重点2"],
    "difficult_points": ["难点1", "难点2"],
    "teaching_methods": ["讲授法", "讨论法"],
    "process": [
        {{"stage": "导入", "duration": 5, "activity": "...", "method": "..."}},
        {{"stage": "新授", "duration": 20, "activity": "...", "method": "..."}},
        {{"stage": "练习", "duration": 10, "activity": "...", "method": "..."}},
        {{"stage": "小结", "duration": 3, "activity": "...", "method": "..."}},
        {{"stage": "作业", "duration": 2, "content": "..."}}
    ],
    "board_design": "板书内容"
}}
"""
```

### 3.2 教案生成流程

```python
# 教案生成 Prompt 设计
LESSON_PLAN_PROMPT = """
基于以下课程大纲，编写详细的教案。

大纲：
{outline_json}

要求：
1. 每个教学环节需包含：
   - 教师活动（具体话术、操作）
   - 学生活动（预期反应、互动方式）
   - 设计意图（为什么这样设计）
   - 时间分配
   - 所需资源（教具、多媒体）

2. 内容要求：
   - 语言生动，适合课堂讲授
   - 包含至少2个互动环节
   - 包含1个探究或实验环节
   - 融入学科核心素养

请以 Markdown 格式输出完整教案。
"""
```

### 3.3 PPT 生成流程

```python
# PPT 结构生成 Prompt
PPT_STRUCTURE_PROMPT = """
基于以下教案内容，设计 PPT 的页面结构。

教案：
{lesson_plan}

要求：
1. 每页 PPT 包含：
   - 页面类型（标题页、内容页、图片页、互动页、总结页）
   - 页面标题
   - 主要内容要点（3-5条）
   - 建议配图描述
   - 动画效果建议

2. 设计原则：
   - 每页内容不宜过多，突出重点
   - 图文比例适中
   - 适当留白
   - 字体大小适中（标题32-44pt，正文18-24pt）

请以 JSON 格式输出：
{{
    "slides": [
        {{
            "type": "title",
            "title": "...",
            "subtitle": "..."
        }},
        {{
            "type": "content",
            "title": "...",
            "bullets": ["要点1", "要点2", "要点3"],
            "image_desc": "配图描述",
            "animation": "淡入"
        }}
    ]
}}
"""
```

---

## 四、代码实现

### 4.1 核心工作流引擎

```python
from typing import Dict, List, Optional
from dataclasses import dataclass
from enum import Enum
import json

class Stage(Enum):
    """备课阶段枚举"""
    OUTLINE = "outline"
    LESSON_PLAN = "lesson_plan"
    PPT = "ppt"
    REVIEW = "review"

@dataclass
class CourseRequest:
    """课程请求参数"""
    subject: str          # 学科
    grade: str            # 年级
    topic: str            # 主题
    duration: int = 45    # 课时（分钟）
    style: str = "standard"  # 风格：standard/interactive/experimental

@dataclass
class WorkflowState:
    """工作流状态"""
    request: CourseRequest
    current_stage: Stage
    outline: Optional[Dict] = None
    lesson_plan: Optional[str] = None
    ppt_path: Optional[str] = None
    errors: List[str] = None
    
    def __post_init__(self):
        if self.errors is None:
            self.errors = []


class LessonPreparationWorkflow:
    """备课工作流主类"""
    
    def __init__(self, llm_client, rag_retriever, template_manager):
        self.llm = llm_client
        self.rag = rag_retriever
        self.templates = template_manager
        self.state: Optional[WorkflowState] = None
    
    async def execute(self, request: CourseRequest) -> WorkflowState:
        """执行完整备课工作流"""
        self.state = WorkflowState(
            request=request,
            current_stage=Stage.OUTLINE
        )
        
        try:
            # Stage 1: 生成大纲
            await self._generate_outline()
            
            # Stage 2: 生成教案
            await self._generate_lesson_plan()
            
            # Stage 3: 生成 PPT
            await self._generate_ppt()
            
            # Stage 4: 质量审核
            await self._review_and_finalize()
            
        except Exception as e:
            self.state.errors.append(str(e))
        
        return self.state
    
    async def _generate_outline(self):
        """生成课程大纲"""
        print(f"[Stage 1] 生成课程大纲...")
        
        # 构建 Prompt
        prompt = self._build_outline_prompt(self.state.request)
        
        # 调用 LLM
        response = await self.llm.generate(prompt)
        
        # 解析 JSON
        try:
            outline = json.loads(response)
            self.state.outline = outline
            self.state.current_stage = Stage.LESSON_PLAN
            print(f"✓ 大纲生成完成: {outline['title']}")
        except json.JSONDecodeError:
            raise ValueError("大纲 JSON 解析失败")
    
    async def _generate_lesson_plan(self):
        """生成详细教案"""
        print(f"[Stage 2] 生成详细教案...")
        
        # RAG 检索相关资源
        context_resources = await self.rag.retrieve(
            query=f"{self.state.request.topic} 教学设计",
            subject=self.state.request.subject,
            grade=self.state.request.grade
        )
        
        # 构建 Prompt
        prompt = self._build_lesson_plan_prompt(
            outline=self.state.outline,
            resources=context_resources
        )
        
        # 调用 LLM
        lesson_plan = await self.llm.generate(prompt)
        self.state.lesson_plan = lesson_plan
        self.state.current_stage = Stage.PPT
        print(f"✓ 教案生成完成")
    
    async def _generate_ppt(self):
        """生成 PPT"""
        print(f"[Stage 3] 生成 PPT...")
        
        # 生成 PPT 结构
        structure_prompt = self._build_ppt_structure_prompt(self.state.lesson_plan)
        structure_json = await self.llm.generate(structure_prompt)
        ppt_structure = json.loads(structure_json)
        
        # 生成 PPT 文件
        ppt_generator = PPTGenerator(self.templates)
        ppt_path = ppt_generator.generate(
            structure=ppt_structure,
            subject=self.state.request.subject,
            style=self.state.request.style
        )
        
        self.state.ppt_path = ppt_path
        self.state.current_stage = Stage.REVIEW
        print(f"✓ PPT 生成完成: {ppt_path}")
    
    async def _review_and_finalize(self):
        """质量审核"""
        print(f"[Stage 4] 质量审核...")
        
        # 内容审核
        review_prompt = f"""
        请审核以下教案内容的质量：
        
        大纲：{json.dumps(self.state.outline, ensure_ascii=False)}
        教案：{self.state.lesson_plan[:1000]}...
        
        检查项：
        1. 内容准确性（是否有知识性错误）
        2. 难度适宜性（是否符合年级水平）
        3. 完整性（是否包含所有必要环节）
        4. 互动性（是否有足够的师生互动）
        
        请以 JSON 格式输出审核结果：
        {{"passed": true/false, "score": 分数, "issues": ["问题1", "问题2"]}}
        """
        
        review_result = await self.llm.generate(review_prompt)
        review = json.loads(review_result)
        
        if not review["passed"]:
            self.state.errors.extend(review["issues"])
        
        print(f"✓ 审核完成，得分: {review['score']}")
    
    def _build_outline_prompt(self, request: CourseRequest) -> str:
        """构建大纲生成 Prompt"""
        return f"""
你是一位经验丰富的{request.subject}教师，请为{request.grade}学生设计一节关于"{request.topic}"的课程大纲。
课时：{request.duration}分钟

请以 JSON 格式输出，包含：title, duration, objectives(知识/能力/情感), 
key_points, difficult_points, teaching_methods, process(导入/新授/练习/小结/作业), board_design
"""
    
    def _build_lesson_plan_prompt(self, outline: Dict, resources: List) -> str:
        """构建教案生成 Prompt"""
        resources_text = "\n".join([f"- {r['title']}: {r['content']}" for r in resources])
        
        return f"""
基于以下大纲编写详细教案：
{json.dumps(outline, ensure_ascii=False)}

参考资源：
{resources_text}

要求：
1. 每个环节包含教师活动、学生活动、设计意图
2. 语言生动，适合课堂讲授
3. 包含至少2个互动环节
4. 以 Markdown 格式输出
"""
    
    def _build_ppt_structure_prompt(self, lesson_plan: str) -> str:
        """构建 PPT 结构生成 Prompt"""
        return f"""
基于以下教案设计 PPT 页面结构：
{lesson_plan[:2000]}...

请以 JSON 格式输出 slides 数组，每个 slide 包含：
type(标题页/内容页/图片页/互动页/总结页), title, bullets(要点列表), image_desc, animation
"""
```

### 4.2 PPT 生成器实现

```python
from pptx import Presentation
from pptx.util import Inches, Pt
from pptx.dml.color import RgbColor
from pptx.enum.text import PP_ALIGN, MSO_ANCHOR
from pptx.enum.shapes import MSO_SHAPE
import os

class PPTGenerator:
    """PPT 生成器"""
    
    def __init__(self, template_manager):
        self.templates = template_manager
        self.subject_styles = {
            "语文": {"primary": RgbColor(0xC4, 0x1E, 0x3A), "secondary": RgbColor(0xF5, 0xF5, 0xDC)},
            "数学": {"primary": RgbColor(0x00, 0x66, 0xCC), "secondary": RgbColor(0xE6, 0xF2, 0xFF)},
            "英语": {"primary": RgbColor(0xFF, 0x66, 0x00), "secondary": RgbColor(0xFF, 0xF0, 0xE6)},
            "物理": {"primary": RgbColor(0x33, 0x99, 0x66), "secondary": RgbColor(0xE6, 0xF5, 0xEE)},
            "化学": {"primary": RgbColor(0x99, 0x33, 0xCC), "secondary": RgbColor(0xF5, 0xEE, 0xFF)},
            "生物": {"primary": RgbColor(0x00, 0x99, 0x66), "secondary": RgbColor(0xE6, 0xF5, 0xEE)},
        }
    
    def generate(self, structure: Dict, subject: str, style: str) -> str:
        """生成 PPT 文件"""
        # 创建演示文稿
        prs = Presentation()
        prs.slide_width = Inches(13.333)
        prs.slide_height = Inches(7.5)
        
        # 获取学科配色
        colors = self.subject_styles.get(subject, self.subject_styles["语文"])
        
        # 遍历页面结构生成幻灯片
        for i, slide_data in enumerate(structure["slides"]):
            slide_type = slide_data["type"]
            
            if slide_type == "title":
                self._add_title_slide(prs, slide_data, colors)
            elif slide_type == "content":
                self._add_content_slide(prs, slide_data, colors)
            elif slide_type == "image":
                self._add_image_slide(prs, slide_data, colors)
            elif slide_type == "interactive":
                self._add_interactive_slide(prs, slide_data, colors)
            elif slide_type == "summary":
                self._add_summary_slide(prs, slide_data, colors)
        
        # 保存文件
        output_path = f"output/{structure['slides'][0]['title']}.pptx"
        os.makedirs("output", exist_ok=True)
        prs.save(output_path)
        
        return output_path
    
    def _add_title_slide(self, prs: Presentation, data: Dict, colors: Dict):
        """添加标题页"""
        slide_layout = prs.slide_layouts[6]  # 空白布局
        slide = prs.slides.add_slide(slide_layout)
        
        # 添加背景色块
        shape = slide.shapes.add_shape(
            MSO_SHAPE.RECTANGLE, Inches(0), Inches(0),
            prs.slide_width, prs.slide_height
        )
        shape.fill.solid()
        shape.fill.fore_color.rgb = colors["secondary"]
        shape.line.fill.background()
        
        # 添加标题
        title_box = slide.shapes.add_textbox(
            Inches(0.5), Inches(2.5), Inches(12.333), Inches(1.5)
        )
        title_frame = title_box.text_frame
        title_frame.text = data["title"]
        title_para = title_frame.paragraphs[0]
        title_para.font.size = Pt(54)
        title_para.font.bold = True
        title_para.font.color.rgb = colors["primary"]
        title_para.alignment = PP_ALIGN.CENTER
        
        # 添加副标题
        if "subtitle" in data:
            subtitle_box = slide.shapes.add_textbox(
                Inches(0.5), Inches(4.2), Inches(12.333), Inches(0.8)
            )
            subtitle_frame = subtitle_box.text_frame
            subtitle_frame.text = data["subtitle"]
            subtitle_para = subtitle_frame.paragraphs[0]
            subtitle_para.font.size = Pt(28)
            subtitle_para.font.color.rgb = RgbColor(0x66, 0x66, 0x66)
            subtitle_para.alignment = PP_ALIGN.CENTER
    
    def _add_content_slide(self, prs: Presentation, data: Dict, colors: Dict):
        """添加内容页"""
        slide_layout = prs.slide_layouts[6]
        slide = prs.slides.add_slide(slide_layout)
        
        # 添加标题栏
        header = slide.shapes.add_shape(
            MSO_SHAPE.RECTANGLE, Inches(0), Inches(0),
            prs.slide_width, Inches(1.2)
        )
        header.fill.solid()
        header.fill.fore_color.rgb = colors["primary"]
        header.line.fill.background()
        
        # 标题文字
        title_box = slide.shapes.add_textbox(
            Inches(0.5), Inches(0.25), Inches(12.333), Inches(0.8)
        )
        title_frame = title_box.text_frame
        title_frame.text = data["title"]
        title_para = title_frame.paragraphs[0]
        title_para.font.size = Pt(36)
        title_para.font.bold = True
        title_para.font.color.rgb = RgbColor(0xFF, 0xFF, 0xFF)
        
        # 添加内容要点
        content_box = slide.shapes.add_textbox(
            Inches(0.8), Inches(1.8), Inches(11.733), Inches(5.2)
        )
        content_frame = content_box.text_frame
        content_frame.word_wrap = True
        
        for i, bullet in enumerate(data.get("bullets", [])):
            if i == 0:
                p = content_frame.paragraphs[0]
            else:
                p = content_frame.add_paragraph()
            
            p.text = f"● {bullet}"
            p.font.size = Pt(24)
            p.font.color.rgb = RgbColor(0x33, 0x33, 0x33)
            p.space_before = Pt(12)
            p.level = 0
```

### 4.3 使用示例

```python
async def main():
    """使用示例"""
    # 初始化组件
    llm_client = OpenAIClient(api_key="your-api-key")
    rag_retriever = RAGRetriever(vector_db_path="./knowledge_base")
    template_manager = TemplateManager(template_dir="./templates")
    
    # 创建工作流
    workflow = LessonPreparationWorkflow(
        llm_client=llm_client,
        rag_retriever=rag_retriever,
        template_manager=template_manager
    )
    
    # 创建课程请求
    request = CourseRequest(
        subject="生物",
        grade="初中八年级",
        topic="光合作用",
        duration=45,
        style="experimental"
    )
    
    # 执行工作流
    result = await workflow.execute(request)
    
    # 输出结果
    print("\n=== 备课结果 ===")
    print(f"课程标题: {result.outline['title']}")
    print(f"PPT 文件: {result.ppt_path}")
    
    if result.errors:
        print(f"\n警告: {len(result.errors)} 个问题")
        for error in result.errors:
            print(f"  - {error}")

# 运行
# asyncio.run(main())
```

---

## 五、面试要点

### 5.1 核心概念

1. **工作流设计原则**：模块化、可扩展、容错性
2. **Prompt 工程**：结构化输出、少样本示例、链式思考
3. **RAG 增强**：本地知识库检索提升内容准确性

### 5.2 常见问题

**Q: 如何保证生成内容的准确性？**
A: 多层保障：① RAG 检索权威资料 ② LLM 自我检查 ③ 人工审核环节

**Q: PPT 生成如何保持风格一致性？**
A: 使用模板引擎 + 学科配色方案 + 统一的排版规则

**Q: 如何处理用户的个性化需求？**
A: 支持风格参数配置 + 模板自定义 + 生成后编辑

---

## 六、参考资源

- [python-pptx 文档](https://python-pptx.readthedocs.io/)
- [LangChain 工作流](https://python.langchain.com/docs/modules/chains/)
- [Prompt Engineering Guide](https://www.promptingguide.ai/)
