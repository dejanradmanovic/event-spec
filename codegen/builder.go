package codegen

import (
	"sort"
	"time"

	"github.com/dejanradmanovic/event-spec/spec"
)

// buildTemplateData converts a slice of EventDef into template-ready data.
// Events are sorted by name for deterministic output.
func buildTemplateData(events []*spec.EventDef, lc LangConfig, workspace, source, pkg string) TemplateData {
	sorted := make([]*spec.EventDef, len(events))
	copy(sorted, events)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	td := TemplateData{
		Workspace:   workspace,
		Source:      source,
		Package:     pkg,
		GeneratedAt: time.Now().UTC(),
		Lang:        lc,
	}
	for _, def := range sorted {
		td.Events = append(td.Events, buildEventData(def, lc))
	}
	return td
}

func buildEventData(def *spec.EventDef, lc LangConfig) EventTemplateData {
	namer := lc.Namer
	className := namer.TypeName(def.Name)

	displayName := def.DisplayName
	if displayName == "" {
		displayName = def.Name
	}
	eventName := def.EventName
	if eventName == "" {
		eventName = def.Name
	}

	ev := EventTemplateData{
		NameRaw:        def.Name,
		NameDisplay:    displayName,
		EventName:      eventName,
		Version:        def.Version,
		Description:    def.Description,
		MethodName:     namer.MethodName(def.Name),
		ClassName:      className,
		ParamsTypeName: className + "Properties",
	}

	for name, prop := range def.Properties {
		pd := buildPropData(name, prop, className, lc)
		if prop.Required {
			ev.RequiredProps = append(ev.RequiredProps, pd)
		} else {
			ev.OptionalProps = append(ev.OptionalProps, pd)
		}
	}

	sort.Slice(ev.RequiredProps, func(i, j int) bool { return ev.RequiredProps[i].NameRaw < ev.RequiredProps[j].NameRaw })
	sort.Slice(ev.OptionalProps, func(i, j int) bool { return ev.OptionalProps[i].NameRaw < ev.OptionalProps[j].NameRaw })

	ev.HasProps = len(ev.RequiredProps)+len(ev.OptionalProps) > 0
	return ev
}

func buildPropData(name string, prop spec.PropertyDef, className string, lc LangConfig) PropTemplateData {
	namer := lc.Namer
	mapper := lc.TypeMapper
	isEnum := len(prop.Enum) > 0 && prop.Type == spec.PropertyTypeString
	enumTypeName := ""
	if isEnum {
		enumTypeName = className + namer.TypeName(name)
	}

	var nativeType string
	if isEnum {
		nativeType = enumTypeName
	} else {
		nativeType = mapper.NativeType(prop)
	}

	return PropTemplateData{
		NameRaw:      name,
		NameField:    namer.FieldName(name),
		TypeNative:   nativeType,
		TypeOptional: mapper.OptionalType(nativeType),
		Required:     prop.Required,
		Enum:         prop.Enum,
		IsEnum:       isEnum,
		EnumTypeName: enumTypeName,
		Description:  prop.Description,
	}
}
