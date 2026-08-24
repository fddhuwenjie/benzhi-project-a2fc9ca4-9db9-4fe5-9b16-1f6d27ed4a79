package domain

import (
	"regexp"
	"strings"
)

var digestPattern = regexp.MustCompile(`^[a-fA-F0-9]{16,128}$`)

func ValidateNewCase(manuscriptCode, title, section string, figures []FigureRecord) error {
	fields := make([]FieldError, 0)
	if strings.TrimSpace(manuscriptCode) == "" {
		fields = append(fields, FieldError{Field: "manuscript_code", Message: "稿件编号不能为空"})
	}
	if strings.TrimSpace(title) == "" {
		fields = append(fields, FieldError{Field: "title", Message: "标题不能为空"})
	}
	if strings.TrimSpace(section) == "" {
		fields = append(fields, FieldError{Field: "journal_section", Message: "期刊栏目不能为空"})
	}
	if err := ValidateFigureList(figures); err != nil {
		if value := AsDomainError(err); value != nil {
			fields = append(fields, value.Fields...)
		}
	}
	if len(fields) > 0 {
		return ValidationError(fields...)
	}
	return nil
}

func ValidateFigureList(figures []FigureRecord) error {
	fields := make([]FieldError, 0)
	if len(figures) == 0 {
		fields = append(fields, FieldError{Field: "figures", Message: "至少登记一幅图像"})
	}
	seen := map[string]bool{}
	for i := range figures {
		prefix := "figures[" + itoa(i) + "]."
		key := strings.ToLower(strings.TrimSpace(figures[i].FigureLabel + ":" + figures[i].PanelLabel))
		if strings.Trim(key, ":") != "" && seen[key] {
			fields = append(fields, FieldError{Field: prefix + "figure_label", Message: "图像编号与面板编号组合必须唯一"})
		}
		seen[key] = true
		if strings.TrimSpace(figures[i].Caption) == "" {
			fields = append(fields, FieldError{Field: prefix + "caption", Message: "图注不能为空"})
		}
		if !digestPattern.MatchString(figures[i].ContentDigest) {
			fields = append(fields, FieldError{Field: prefix + "content_digest", Message: "文件摘要须为 16 到 128 位十六进制字符"})
		}
		if figures[i].PixelWidth <= 0 || figures[i].PixelHeight <= 0 {
			fields = append(fields, FieldError{Field: prefix + "dimensions", Message: "图像尺寸必须为正整数"})
		}
	}
	if len(fields) > 0 {
		return ValidationError(fields...)
	}
	return nil
}

func NormalizeFigure(figure FigureRecord) FigureRecord {
	figure.ID = strings.TrimSpace(figure.ID)
	figure.FigureLabel = strings.TrimSpace(figure.FigureLabel)
	figure.PanelLabel = strings.TrimSpace(figure.PanelLabel)
	figure.Caption = strings.TrimSpace(figure.Caption)
	figure.ContentDigest = strings.ToLower(strings.TrimSpace(figure.ContentDigest))
	figure.ExperimentSource = strings.TrimSpace(figure.ExperimentSource)
	figure.RawDataReference = strings.TrimSpace(figure.RawDataReference)
	return figure
}

func ValidateRequest(requestID string, expectedRevision int64) error {
	fields := make([]FieldError, 0, 2)
	if strings.TrimSpace(requestID) == "" || len(requestID) > 128 {
		fields = append(fields, FieldError{Field: "request_id", Message: "请求标识不能为空且不能超过 128 个字符"})
	}
	if expectedRevision < 0 {
		fields = append(fields, FieldError{Field: "expected_revision", Message: "revision 不能为负数"})
	}
	if len(fields) > 0 {
		return ValidationError(fields...)
	}
	return nil
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	result := ""
	for value > 0 {
		result = string(rune('0'+value%10)) + result
		value /= 10
	}
	return result
}
