package foodflow

import "strings"

func (m foodModel) renderHome(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" Food API")))
	if m.selectedAddress == nil {
		sb.WriteString(line(" context address_id=<required>"))
	} else {
		sb.WriteString(line(" context address_id=" + boldStyle.Render(m.selectedAddress.ID)))
	}
	for i, choice := range foodHomeChoices {
		label := foodHomeChoiceLabel(choice)
		if m.homeCursor == i {
			sb.WriteString(line(cursorStyle.Render("> ") + boldStyle.Render(label)))
		} else {
			sb.WriteString(line("   " + label))
		}
	}
}

func foodHomeChoiceLabel(choice foodHomeChoice) string {
	if strings.TrimSpace(choice.icon) == "" {
		return choice.label
	}
	return choice.icon + "  " + choice.label
}
