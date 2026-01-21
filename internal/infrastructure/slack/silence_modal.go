package slack

import (
	"fmt"
	"sort"

	"github.com/slack-go/slack"
)

// SilenceModalCallbackID is the callback ID for the silence modal submission.
const SilenceModalCallbackID = "silence_create_modal"

// Silence modal block IDs
const (
	SilenceBlockDuration = "silence_duration"
	SilenceBlockReason   = "silence_reason"
	SilenceBlockMatchers = "silence_matchers"
)

// Silence modal action IDs
const (
	SilenceActionDuration = "silence_duration_select"
	SilenceActionReason   = "silence_reason_input"
	SilenceActionMatchers = "silence_matchers_select"
)

// DurationOption represents a silence duration option.
type DurationOption struct {
	Label string
	Value string
}

// DefaultDurationOptions returns the default silence duration options.
func DefaultDurationOptions() []DurationOption {
	return []DurationOption{
		{Label: "30 minutes", Value: "30m"},
		{Label: "1 hour", Value: "1h"},
		{Label: "2 hours", Value: "2h"},
		{Label: "4 hours", Value: "4h"},
		{Label: "8 hours", Value: "8h"},
		{Label: "24 hours", Value: "24h"},
		{Label: "3 days", Value: "72h"},
		{Label: "1 week", Value: "168h"},
	}
}

// BuildSilenceModal creates a modal view for creating a silence.
// labelOptions is a map of label keys to their possible values.
func BuildSilenceModal(labelOptions map[string][]string) slack.ModalViewRequest {
	// Title
	titleText := slack.NewTextBlockObject(slack.PlainTextType, "Create Silence", false, false)

	// Submit button
	submitText := slack.NewTextBlockObject(slack.PlainTextType, "Create", false, false)

	// Close button
	closeText := slack.NewTextBlockObject(slack.PlainTextType, "Cancel", false, false)

	// Build blocks
	blocks := slack.Blocks{
		BlockSet: []slack.Block{},
	}

	// Duration select
	durationOptions := buildDurationOptions()
	durationSelect := slack.NewOptionsSelectBlockElement(
		slack.OptTypeStatic,
		slack.NewTextBlockObject(slack.PlainTextType, "Select duration", false, false),
		SilenceActionDuration,
		durationOptions...,
	)
	// Set default to 1 hour
	durationSelect.InitialOption = durationOptions[1]

	durationInput := slack.NewInputBlock(
		SilenceBlockDuration,
		slack.NewTextBlockObject(slack.PlainTextType, "Duration", false, false),
		nil,
		durationSelect,
	)
	blocks.BlockSet = append(blocks.BlockSet, durationInput)

	// Reason input (optional)
	reasonInput := slack.NewPlainTextInputBlockElement(
		slack.NewTextBlockObject(slack.PlainTextType, "e.g., Planned maintenance", false, false),
		SilenceActionReason,
	)
	reasonInput.Multiline = false
	reasonBlock := slack.NewInputBlock(
		SilenceBlockReason,
		slack.NewTextBlockObject(slack.PlainTextType, "Reason (optional)", false, false),
		nil,
		reasonInput,
	)
	reasonBlock.Optional = true
	blocks.BlockSet = append(blocks.BlockSet, reasonBlock)

	// Label matchers section
	if len(labelOptions) > 0 {
		// Add divider
		blocks.BlockSet = append(blocks.BlockSet, slack.NewDividerBlock())

		// Add header for matchers
		matcherHeader := slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType,
				"*Label Matchers* (optional)\nSelect labels to match specific alerts",
				false, false),
			nil, nil,
		)
		blocks.BlockSet = append(blocks.BlockSet, matcherHeader)

		// Add a multi-select for each label key (up to 5 most common)
		matcherBlocks := buildMatcherBlocks(labelOptions)
		blocks.BlockSet = append(blocks.BlockSet, matcherBlocks...)
	}

	return slack.ModalViewRequest{
		Type:            slack.VTModal,
		CallbackID:      SilenceModalCallbackID,
		Title:           titleText,
		Submit:          submitText,
		Close:           closeText,
		Blocks:          blocks,
		ClearOnClose:    true,
		NotifyOnClose:   false,
		PrivateMetadata: "", // Can be used to pass data between interactions
	}
}

// buildDurationOptions creates the duration select options.
func buildDurationOptions() []*slack.OptionBlockObject {
	options := DefaultDurationOptions()
	result := make([]*slack.OptionBlockObject, len(options))

	for i, opt := range options {
		result[i] = slack.NewOptionBlockObject(
			opt.Value,
			slack.NewTextBlockObject(slack.PlainTextType, opt.Label, false, false),
			nil,
		)
	}

	return result
}

// buildMatcherBlocks creates input blocks for label matchers.
func buildMatcherBlocks(labelOptions map[string][]string) []slack.Block {
	blocks := []slack.Block{}

	// Sort label keys for consistent ordering
	keys := make([]string, 0, len(labelOptions))
	for key := range labelOptions {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Prioritize common labels
	priorityLabels := []string{"alertname", "severity", "instance", "job", "service"}
	sortedKeys := make([]string, 0, len(keys))

	// Add priority labels first
	for _, pl := range priorityLabels {
		for i, k := range keys {
			if k == pl {
				sortedKeys = append(sortedKeys, k)
				keys = append(keys[:i], keys[i+1:]...)
				break
			}
		}
	}
	// Add remaining keys
	sortedKeys = append(sortedKeys, keys...)

	// Limit to 5 label matchers to keep modal manageable
	maxMatchers := 5
	if len(sortedKeys) < maxMatchers {
		maxMatchers = len(sortedKeys)
	}

	for i := 0; i < maxMatchers; i++ {
		key := sortedKeys[i]
		values := labelOptions[key]

		// Sort values
		sort.Strings(values)

		// Create options for this label
		options := make([]*slack.OptionBlockObject, len(values))
		for j, value := range values {
			options[j] = slack.NewOptionBlockObject(
				fmt.Sprintf("%s=%s", key, value),
				slack.NewTextBlockObject(slack.PlainTextType, value, false, false),
				nil,
			)
		}

		// Create multi-select for this label
		selectElement := slack.NewOptionsMultiSelectBlockElement(
			slack.MultiOptTypeStatic,
			slack.NewTextBlockObject(slack.PlainTextType, fmt.Sprintf("Select %s values", key), false, false),
			fmt.Sprintf("%s_%s", SilenceActionMatchers, key),
			options...,
		)

		inputBlock := slack.NewInputBlock(
			fmt.Sprintf("%s_%s", SilenceBlockMatchers, key),
			slack.NewTextBlockObject(slack.PlainTextType, key, false, false),
			nil,
			selectElement,
		)
		inputBlock.Optional = true

		blocks = append(blocks, inputBlock)
	}

	return blocks
}
