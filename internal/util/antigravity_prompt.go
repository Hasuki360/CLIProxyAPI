package util

import (
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var (
	// 匹配 <identity>...You are Antigravity...</identity>（包括换行与大小写）
	reAntigravityIdentityTag = regexp.MustCompile(`(?is)<\s*identity\s*>.*?You are Antigravity.*?<\s*/\s*identity\s*>`)

	// 匹配 <communication_style>...</communication_style>
	reCommunicationStyleTag = regexp.MustCompile(`(?is)<\s*communication_style\s*>.*?<\s*/\s*communication_style\s*>`)

	// 匹配 Sub2API 的静默边界指令
	reSub2APIIdentityBoundary = regexp.MustCompile(`(?is)Below are your system instructions\. Follow them strictly\..*?Never mention ["']Antigravity["'].*?\n`)

	// 匹配系统提示词结尾标记 --- [SYSTEM_PROMPT_END] ---
	reSystemPromptEnd = regexp.MustCompile(`(?i)---\s*\[SYSTEM_PROMPT_END\]\s*---`)

	// 匹配 [IDENTITY_PATCH] 标记
	reIdentityPatchTag = regexp.MustCompile(`(?i)\[IDENTITY_PATCH\]`)

	// 匹配 Please ignore following [ignore]...[/ignore]
	reIgnoreWrapper = regexp.MustCompile(`(?is)Please ignore following \[ignore\].*?\[/ignore\]`)

	// 匹配纯文本形式的 "You are Antigravity, a powerful agentic AI coding assistant..."
	rePlainAntigravityIdentity = regexp.MustCompile(`(?is)You are Antigravity, a powerful agentic AI coding assistant.*?(?:\*\*Proactiveness\*\*|<\s*/\s*communication_style\s*>|\n\n|$)`)
)

// CleanAntigravityIdentityText strips known Antigravity identity prefix prompts,
// communication styles, isolation boundaries, and tags from the input string.
func CleanAntigravityIdentityText(text string) string {
	if text == "" {
		return ""
	}
	// Fast path: if "antigravity" and other keywords are not present, return as-is
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "antigravity") && !strings.Contains(lower, "system_prompt_end") && !strings.Contains(lower, "identity_patch") && !strings.Contains(lower, "[ignore]") {
		return text
	}

	res := text
	res = reAntigravityIdentityTag.ReplaceAllString(res, "")
	res = reCommunicationStyleTag.ReplaceAllString(res, "")
	res = reSub2APIIdentityBoundary.ReplaceAllString(res, "")
	res = reSystemPromptEnd.ReplaceAllString(res, "")
	res = reIdentityPatchTag.ReplaceAllString(res, "")
	res = reIgnoreWrapper.ReplaceAllString(res, "")
	res = rePlainAntigravityIdentity.ReplaceAllString(res, "")

	return strings.TrimSpace(res)
}

// StripAntigravityIdentityPrompt removes Antigravity identity system instructions
// from Gemini/Antigravity request payloads.
func StripAntigravityIdentityPrompt(payload []byte) []byte {
	if len(payload) == 0 {
		return payload
	}

	updated := payload
	paths := []string{"request.systemInstruction", "systemInstruction"}

	for _, sysPath := range paths {
		sysResult := gjson.GetBytes(updated, sysPath)
		if !sysResult.Exists() {
			continue
		}

		partsResult := sysResult.Get("parts")
		if partsResult.IsArray() {
			var keptParts []string
			changed := false
			parts := partsResult.Array()

			for _, part := range parts {
				textResult := part.Get("text")
				if textResult.Type == gjson.String {
					originalText := textResult.String()
					cleanedText := CleanAntigravityIdentityText(originalText)
					if cleanedText != originalText {
						changed = true
						if cleanedText == "" {
							// 该 part 仅包含 Antigravity 身份词，直接剔除
							continue
						}
						// 还包含其他用户自定义指令，保留清理后的文本
						newPart, err := sjson.SetBytes([]byte(part.Raw), "text", cleanedText)
						if err == nil {
							keptParts = append(keptParts, string(newPart))
							continue
						}
					}
				}
				keptParts = append(keptParts, part.Raw)
			}

			if changed {
				if len(keptParts) == 0 {
					// 所有 parts 均被移除，安全删除整个 systemInstruction 结构体
					updated, _ = sjson.DeleteBytes(updated, sysPath)
				} else {
					// 重新写回过滤后的 parts 列表
					partsJSON := "[" + strings.Join(keptParts, ",") + "]"
					updated, _ = sjson.SetRawBytes(updated, sysPath+".parts", []byte(partsJSON))
				}
			}
		}
	}

	// 兼容部分客户端将前置提示词注入到 contents 首条 user 消息的情况
	contentsPaths := []string{"request.contents", "contents"}
	for _, cPath := range contentsPaths {
		firstPartText := gjson.GetBytes(updated, cPath+".0.parts.0.text")
		if firstPartText.Exists() && firstPartText.Type == gjson.String {
			orig := firstPartText.String()
			cleaned := CleanAntigravityIdentityText(orig)
			if cleaned != orig && cleaned != "" {
				updated, _ = sjson.SetBytes(updated, cPath+".0.parts.0.text", cleaned)
			}
		}
	}

	return updated
}
