package instamartflow

import "strings"

func (m instamartModel) renderStatic(sb *strings.Builder) {
	for i, choice := range instamartHomeChoices {
		label := instamartHomeChoiceLabel(choice)
		if m.cursor == i {
			sb.WriteString(line(cursorStyle.Render("> ") + boldStyle.Render(label)))
		} else {
			sb.WriteString(line("   " + label))
		}
	}
}

func (m instamartModel) renderAddresses(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" select deployment address")))
	sb.WriteString(line(""))
	for i, address := range m.addresses {
		label := "⌂  " + addressLabel(address)
		if address.PhoneMasked != "" {
			label += " · " + address.PhoneMasked
		}
		lineText := label + " — " + redactLine(address.DisplayLine)
		if m.cursor == i {
			sb.WriteString(line(cursorStyle.Render("> ") + boldStyle.Render(lineText)))
		} else {
			sb.WriteString(line("   " + lineText))
		}
	}
}

func (m instamartModel) renderHome(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" What would you like to deploy?")))
	if m.selectedAddress == nil {
		sb.WriteString(line(" Choose an address before searching or checkout."))
	} else {
		sb.WriteString(line(""))
	}
	for i, choice := range instamartHomeChoices {
		label := instamartHomeChoiceLabel(choice)
		if m.homeCursor == i {
			sb.WriteString(line(cursorStyle.Render("> ") + boldStyle.Render(label)))
		} else {
			sb.WriteString(line("   " + label))
		}
	}
}

func (m instamartModel) renderHelp(sb *strings.Builder) {
	sb.WriteString(line(brandStyle.Render(" swiggy.dev keys")))
	sb.WriteString(line(""))
	sb.WriteString(line(" j/k        move"))
	sb.WriteString(line(" /          grep products"))
	sb.WriteString(line(" c          staged cart"))
	sb.WriteString(line(" enter      choose"))
	sb.WriteString(line(" +/-        change quantity"))
	sb.WriteString(line(" p          ship from cart"))
	sb.WriteString(line(" b          back home"))
	sb.WriteString(line(" q          quit"))
}

func instamartHomeChoiceLabel(choice instamartHomeChoice) string {
	if strings.TrimSpace(choice.icon) == "" {
		return choice.label
	}
	return choice.icon + "  " + choice.label
}
