package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const IntegrityRuleVersion = "integrity-rules-2026.1"

type pendingFinding struct {
	code       string
	severity   Severity
	figureIDs  []string
	evidence   string
	stableSort string
}

func EvaluateIntegrity(caseID string, figures []FigureRecord, now time.Time) []RiskFinding {
	pending := make([]pendingFinding, 0)
	digests := map[string][]string{}
	groups := map[string][]FigureRecord{}
	for _, figure := range figures {
		if strings.TrimSpace(figure.FigureLabel) == "" {
			pending = append(pending, pendingFinding{"missing_figure_label", SeverityHigh, []string{figure.ID}, "图像编号缺失", figure.ID})
		}
		missing := make([]string, 0, 2)
		if strings.TrimSpace(figure.ExperimentSource) == "" {
			missing = append(missing, "实验来源")
		}
		if strings.TrimSpace(figure.RawDataReference) == "" {
			missing = append(missing, "原始数据声明")
		}
		if len(missing) > 0 {
			pending = append(pending, pendingFinding{"incomplete_provenance", SeverityMedium, []string{figure.ID}, strings.Join(missing, "、") + "不完整", figure.ID})
		}
		digest := strings.ToLower(strings.TrimSpace(figure.ContentDigest))
		digests[digest] = append(digests[digest], figure.ID)
		group := strings.ToLower(strings.TrimSpace(figure.FigureLabel))
		if group != "" {
			groups[group] = append(groups[group], figure)
		}
	}
	for digest, ids := range digests {
		if digest != "" && len(ids) > 1 {
			sort.Strings(ids)
			pending = append(pending, pendingFinding{"duplicate_digest", SeverityCritical, ids, "多个图像使用相同文件摘要 " + digest, strings.Join(ids, ",")})
		}
	}
	for label, members := range groups {
		if len(members) < 2 {
			continue
		}
		minArea, maxArea := members[0].PixelWidth*members[0].PixelHeight, members[0].PixelWidth*members[0].PixelHeight
		ids := make([]string, 0, len(members))
		for _, member := range members {
			area := member.PixelWidth * member.PixelHeight
			if area < minArea {
				minArea = area
			}
			if area > maxArea {
				maxArea = area
			}
			ids = append(ids, member.ID)
		}
		if minArea > 0 && maxArea > minArea*4 {
			sort.Strings(ids)
			pending = append(pending, pendingFinding{"group_dimension_anomaly", SeverityMedium, ids, fmt.Sprintf("同组 %s 的最大像素面积超过最小值四倍", label), strings.Join(ids, ",")})
		}
	}
	sort.Slice(pending, func(i, j int) bool {
		if pending[i].code == pending[j].code {
			return pending[i].stableSort < pending[j].stableSort
		}
		return pending[i].code < pending[j].code
	})
	result := make([]RiskFinding, len(pending))
	for i, item := range pending {
		result[i] = RiskFinding{
			ID:             caseID + "-risk-" + itoa(i+1),
			CaseID:         caseID,
			FigureIDs:      item.figureIDs,
			RuleCode:       item.code,
			RuleVersion:    IntegrityRuleVersion,
			Severity:       item.severity,
			Evidence:       item.evidence,
			ResponseStatus: ResponseNotRequired,
			UpdatedAt:      now.UTC(),
		}
	}
	return result
}
