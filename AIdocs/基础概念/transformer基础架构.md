### 目标：面试能讲清 Transformer，并能落到“推理优化/工程实现”

你们项目里本地大模型（如 Qwen/QwQ）本质上是 **Transformer Decoder-only** 架构的工程化实现。面试里通常不只问“原理”，还会深挖到：

- 推理为什么慢？怎么优化（KV Cache、PagedAttention、量化）？
- 为什么需要做上下文窗口控制？与注意力复杂度 \(O(n^2)\) 的关系？
- 为什么流式输出（SSE/WS）天然适配 LLM？

---

### 1）Transformer（Decoder-only LLM）核心模块

#### 1.1 输入、Embedding 与位置编码

- Tokenize：把文本转成 token ids。
- Embedding：`E[token_id] -> d_model` 向量。
- 位置编码：主流 LLM 常用 **RoPE**（旋转位置编码）来编码顺序。

#### 1.2 自注意力（Self-Attention）

给定输入 `X`：

- `Q = XWq`
- `K = XWk`
- `V = XWv`

\[
Attention(Q,K,V)=softmax(\frac{QK^T}{\sqrt{d_k}} + mask)\,V
\]

Decoder-only 使用 **causal mask**，保证位置 \(t\) 只看见 \(<t\) 的 token。

#### 1.3 多头注意力（MHA）与 FFN

- 多头：把 `d_model` 切成多个 head 并行算注意力，再拼接回去。
- FFN：两层线性 + 激活（常见 SwiGLU），提升非线性表达能力。

#### 1.4 残差连接与 LayerNorm

每层结构大致为：

- `X = X + Attention(LN(X))`
- `X = X + FFN(LN(X))`

---

### 2）工程落地：为什么推理慢？怎么优化（与你们项目强相关）

#### 2.1 注意力复杂度与“上下文记忆”设计

- 注意力需要计算 `QK^T`，序列长度为 `n` 时复杂度约为 \(O(n^2)\)。
- 所以教学对话助手必须用：
  - **窗口裁剪**（保留最近 N 轮原文）
  - **摘要压缩**（结构化 summary）
  - **检索记忆**（RAG/向量库/图谱）替代全量历史

#### 2.2 KV Cache（自回归生成的关键优化）

- LLM 逐 token 生成时，历史 token 的 `K/V` 不必重复计算，可缓存为 **KV Cache**。
- 工程意义：
  - 同一个回答生成过程，token 越往后收益越大。
  - 流式输出时服务端持续复用 KV cache，能明显降低延迟。

#### 2.3 PagedAttention / 连续批处理（vLLM 思路）

- 多用户并发会导致 KV cache 显存碎片化与浪费。
- vLLM 的 PagedAttention 用“分页管理”KV cache，提高吞吐，适合课堂高峰并发。

#### 2.4 量化（QLoRA/INT8/INT4）与成本

你们材料强调 QLoRA 的“低显存”优势，工程上通常这样组合：

- 微调：QLoRA（低秩适配 + 低比特权重量化）
- 推理：INT8/INT4 量化（具体后端看推理栈），在成本与效果间做取舍

---

### 3）面试常见深挖点（建议准备的答案）

#### Q1：为什么流式输出（SSE/WebSocket）天然适合 LLM？

- 自回归生成是一 token 一 token 产出，服务端可以边生成边返回 chunk。
- 支持中途取消，显著节省算力成本。

#### Q2：为什么需要 RAG？它与 Transformer 的关系是什么？

- Transformer 只是在已有上下文上做概率生成；RAG 用检索把“外部真实资料”补进上下文。
- 教育场景要降低幻觉，RAG + 引用来源输出是最常见工程路径。

#### Q3：KV Cache 能跨多轮对话复用吗？

- 一次生成内部复用（强相关）。
- 跨请求复用一般不做（需要精确对齐 token 序列与状态），工程上更常用“摘要 + RAG”来解决长对话。

