package main

import core "shuttle/internal/core/inventory"

// Bridge DTOs avoid anonymous embedded fields in generated Wails models.
type InventoryStep struct {
	Item        InventoryItem
	Action      string
	Command     string
	Instruction string
}

type InventoryPlan struct {
	Steps     []InventoryStep
	Blocked   bool
	Conflicts []string
}

func inventoryPlanView(plan core.Plan) InventoryPlan {
	out := InventoryPlan{Blocked: plan.Blocked, Conflicts: plan.Conflicts, Steps: make([]InventoryStep, 0, len(plan.Steps))}
	for _, step := range plan.Steps {
		item := step.Item
		out.Steps = append(out.Steps, InventoryStep{Item: InventoryItem{Name: item.Name, Version: item.Version, Source: string(item.Source), Root: item.Root, Ownership: string(item.Ownership), Reason: item.Reason}, Action: step.Action, Command: step.Command, Instruction: step.Instruction})
	}
	return out
}
