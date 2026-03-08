// Package llm provides LLM provider implementations for zen-brain.
package llm

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/llm"
)

// PlannerProvider implements the Provider interface for planner/escalation lane.
// This provider handles complex planning tasks using more powerful (often cloud) models.
type PlannerProvider struct {
	model   string
	timeout int // seconds
}

// NewPlannerProvider creates a new PlannerProvider.
func NewPlannerProvider(model string, timeout int) *PlannerProvider {
	return &PlannerProvider{
		model:   model,
		timeout: timeout,
	}
}

// Name returns the provider name.
func (p *PlannerProvider) Name() string {
	return "planner"
}

// SupportsTools returns true if planner supports tools.
func (p *PlannerProvider) SupportsTools() bool {
	return true // Planner lane supports tools
}

// Chat sends a chat request to the planner.
func (p *PlannerProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	startTime := time.Now()
	
	// Apply provider timeout
	timeout := time.Duration(p.timeout) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	log.Printf("[Planner] Processing complex chat request: model=%s, messages=%d, tools=%d, task_id=%s",
		p.model, len(req.Messages), len(req.Tools), req.TaskID)

	// Simulate cloud model processing with context awareness
	// In production, this would call OpenAI, Anthropic, or similar
	select {
	case <-time.After(200 * time.Millisecond): // Simulate cloud latency
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Generate a comprehensive, planning-oriented response
	content := generatePlannerResponse(req)
	
	// Check if tools were requested - planner is more likely to use tools
	var toolCalls []llm.ToolCall
	if len(req.Tools) > 0 && shouldCallPlannerTools(req) {
		toolCalls = generatePlannerToolCalls(req)
	}

	// Planner can generate reasoning content
	reasoningContent := generatePlannerReasoning(req)

	latency := time.Since(startTime).Milliseconds()

	resp := &llm.ChatResponse{
		Content:         content,
		ReasoningContent: reasoningContent,
		FinishReason:    "stop",
		Model:           p.model,
		ToolCalls:       toolCalls,
		Usage: &llm.TokenUsage{
			InputTokens:  estimatePlannerTokens(req.Messages),
			OutputTokens: estimateTokensFromContent(content) + estimateTokensFromContent(reasoningContent),
			TotalTokens:  estimatePlannerTokens(req.Messages) + estimateTokensFromContent(content) + estimateTokensFromContent(reasoningContent),
		},
		LatencyMs: latency,
	}

	log.Printf("[Planner] Complex response generated: latency=%dms, tokens=%d, reasoning=%d chars",
		latency, resp.Usage.TotalTokens, len(reasoningContent))

	return resp, nil
}

// ChatStream sends a streaming chat request.
// For MVP, falls back to non-streaming Chat.
func (p *PlannerProvider) ChatStream(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) (*llm.ChatResponse, error) {
	// Planner doesn't support streaming in MVP
	// Fall back to regular chat
	return p.Chat(ctx, req)
}

// Embed generates an embedding.
// Planner doesn't support embeddings in MVP.
func (p *PlannerProvider) Embed(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	return nil, llm.ErrEmbeddingNotSupported
}

// generatePlannerResponse generates a comprehensive response for the planner lane.
func generatePlannerResponse(req llm.ChatRequest) string {
	if len(req.Messages) == 0 {
		return "As the planning agent, I need to understand the context and requirements to provide strategic guidance."
	}

	lastMessage := req.Messages[len(req.Messages)-1].Content
	
	// Check for specific planning-related queries
	lowerMsg := strings.ToLower(lastMessage)
	
	switch {
	case strings.Contains(lowerMsg, "plan") || strings.Contains(lowerMsg, "strategy"):
		return `Based on your request, I recommend a phased approach:

1. **Analysis Phase** (1-2 hours): Understand requirements and constraints
2. **Design Phase** (2-3 hours): Create detailed specifications  
3. **Implementation Phase** (4-8 hours): Execute with frequent validation
4. **Review Phase** (1-2 hours): Quality assurance and documentation

Key considerations:
- Risk assessment: Medium complexity, requires careful testing
- Estimated cost: $X.XX USD (varies based on model usage)
- Success probability: 85% based on similar tasks

Would you like me to proceed with this plan or adjust any aspects?`

	case strings.Contains(lowerMsg, "analyze") || strings.Contains(lowerMsg, "review"):
		return `I've analyzed the situation and identified several key factors:

**Strengths:**
- Clear objectives stated
- Existing infrastructure available
- Modular design possible

**Risks:**
- Potential integration challenges
- Time constraints may require prioritization
- Dependencies on external systems

**Recommendations:**
1. Start with a proof-of-concept for riskiest components
2. Implement monitoring from day one
3. Schedule regular checkpoints for course correction

The analysis suggests a 75% confidence level for successful completion within estimated parameters.`

	case strings.Contains(lowerMsg, "architecture") || strings.Contains(lowerMsg, "design"):
		return `Proposed architecture:

**High-Level Components:**
- Frontend: React with TypeScript for maintainability
- Backend: Go services with gRPC for performance
- Database: PostgreSQL with read replicas
- Cache: Redis for session management

**Key Design Decisions:**
1. Microservices over monolith for scalability
2. Event-driven architecture for loose coupling
3. Comprehensive logging and monitoring
4. Automated testing at all levels

**Implementation Timeline:**
- Week 1-2: Core infrastructure setup
- Week 3-4: Basic functionality
- Week 5-6: Advanced features
- Week 7-8: Testing and deployment

This design balances complexity with maintainability.`

	case strings.Contains(lowerMsg, "help") || strings.Contains(lowerMsg, "assist"):
		return "As the planning agent, I specialize in complex problem-solving, architectural design, strategic planning, and risk assessment. I can help you break down complex tasks, estimate costs and timelines, identify risks, and create actionable plans. What specific challenge would you like to address?"

	default:
		return fmt.Sprintf(`I've processed your complex request regarding: "%s"

**As the planning agent, my assessment:**
- This appears to be a medium-complexity task requiring careful planning
- Estimated effort: 3-5 hours of focused work
- Recommended approach: Iterative development with frequent validation
- Key success factor: Clear requirements and regular feedback loops

**Next steps I recommend:**
1. Formalize requirements in a structured document
2. Identify potential risks and mitigation strategies
3. Create a phased implementation plan
4. Establish success metrics and checkpoints

Would you like me to elaborate on any of these aspects or proceed with creating a detailed plan?`, 
			truncateString(lastMessage, 150))
	}
}

// generatePlannerReasoning generates chain-of-thought reasoning for planner.
func generatePlannerReasoning(req llm.ChatRequest) string {
	// Generate reasoning content that shows the planner's thought process
	return `Reasoning process:
1. First, I analyzed the request to determine its complexity level and domain.
2. I considered the available tools and constraints mentioned in the context.
3. Based on past similar tasks, I estimated the effort and potential challenges.
4. I structured the response to provide actionable advice while acknowledging uncertainties.
5. The recommendation balances thoroughness with practicality for execution.`
}

// shouldCallPlannerTools determines if planner should call tools.
func shouldCallPlannerTools(req llm.ChatRequest) bool {
	if len(req.Tools) == 0 {
		return false
	}

	// Planner is more likely to use tools for complex tasks
	lastMessage := ""
	if len(req.Messages) > 0 {
		lastMessage = strings.ToLower(req.Messages[len(req.Messages)-1].Content)
	}

	// Planner uses tools for execution, analysis, or data gathering
	toolKeywords := []string{"execute", "implement", "analyze", "fetch", "get data", "process", "calculate"}
	for _, keyword := range toolKeywords {
		if strings.Contains(lastMessage, keyword) {
			return true
		}
	}

	// Also use tools if this is a task execution request
	if req.TaskID != "" && req.SessionID != "" {
		return true
	}

	return false
}

// generatePlannerToolCalls generates tool calls for complex planning tasks.
func generatePlannerToolCalls(req llm.ChatRequest) []llm.ToolCall {
	if len(req.Tools) == 0 {
		return nil
	}

	// Planner might call multiple tools for complex tasks
	toolCalls := make([]llm.ToolCall, 0, min(3, len(req.Tools)))
	
	for i, tool := range req.Tools {
		if i >= 3 { // Limit to 3 tool calls
			break
		}

		// Generate appropriate args based on tool type
		args := map[string]interface{}{
			"task_id":    req.TaskID,
			"session_id": req.SessionID,
		}

		// Add tool-specific arguments
		switch {
		case strings.Contains(strings.ToLower(tool.Name), "analyze"):
			args["depth"] = "detailed"
			args["include_risks"] = true
		case strings.Contains(strings.ToLower(tool.Name), "execute"):
			args["phase"] = "planning"
			args["validate"] = true
		case strings.Contains(strings.ToLower(tool.Name), "fetch"):
			args["source"] = "primary"
			args["format"] = "structured"
		default:
			args["action"] = "process"
		}

		toolCalls = append(toolCalls, llm.ToolCall{
			ID:   fmt.Sprintf("planner_call_%d_%d", time.Now().UnixNano(), i),
			Name: tool.Name,
			Args: args,
		})
	}

	return toolCalls
}

// estimatePlannerTokens estimates token count with overhead for complex thinking.
func estimatePlannerTokens(messages []llm.Message) int64 {
	total := 0
	for _, msg := range messages {
		total += len(msg.Content)
		if msg.ReasoningContent != "" {
			total += len(msg.ReasoningContent)
		}
	}
	// Planner uses more tokens for internal reasoning
	// Add 20% overhead for complex processing
	adjustedTotal := float64(total) * 1.2
	return int64(adjustedTotal / 4) // 4 chars per token
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}