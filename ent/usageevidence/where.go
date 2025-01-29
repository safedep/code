// Code generated by ent, DO NOT EDIT.

package usageevidence

import (
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
	"github.com/safedep/code/ent/predicate"
)

// ID filters vertices based on their ID field.
func ID(id int) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEQ(FieldID, id))
}

// IDEQ applies the EQ predicate on the ID field.
func IDEQ(id int) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEQ(FieldID, id))
}

// IDNEQ applies the NEQ predicate on the ID field.
func IDNEQ(id int) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldNEQ(FieldID, id))
}

// IDIn applies the In predicate on the ID field.
func IDIn(ids ...int) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldIn(FieldID, ids...))
}

// IDNotIn applies the NotIn predicate on the ID field.
func IDNotIn(ids ...int) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldNotIn(FieldID, ids...))
}

// IDGT applies the GT predicate on the ID field.
func IDGT(id int) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldGT(FieldID, id))
}

// IDGTE applies the GTE predicate on the ID field.
func IDGTE(id int) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldGTE(FieldID, id))
}

// IDLT applies the LT predicate on the ID field.
func IDLT(id int) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldLT(FieldID, id))
}

// IDLTE applies the LTE predicate on the ID field.
func IDLTE(id int) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldLTE(FieldID, id))
}

// PackageHint applies equality check predicate on the "PackageHint" field. It's identical to PackageHintEQ.
func PackageHint(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEQ(FieldPackageHint, v))
}

// ModuleName applies equality check predicate on the "ModuleName" field. It's identical to ModuleNameEQ.
func ModuleName(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEQ(FieldModuleName, v))
}

// ModuleItem applies equality check predicate on the "ModuleItem" field. It's identical to ModuleItemEQ.
func ModuleItem(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEQ(FieldModuleItem, v))
}

// ModuleAlias applies equality check predicate on the "ModuleAlias" field. It's identical to ModuleAliasEQ.
func ModuleAlias(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEQ(FieldModuleAlias, v))
}

// IsWildCardUsage applies equality check predicate on the "IsWildCardUsage" field. It's identical to IsWildCardUsageEQ.
func IsWildCardUsage(v bool) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEQ(FieldIsWildCardUsage, v))
}

// Identifier applies equality check predicate on the "Identifier" field. It's identical to IdentifierEQ.
func Identifier(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEQ(FieldIdentifier, v))
}

// UsageFilePath applies equality check predicate on the "UsageFilePath" field. It's identical to UsageFilePathEQ.
func UsageFilePath(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEQ(FieldUsageFilePath, v))
}

// Line applies equality check predicate on the "Line" field. It's identical to LineEQ.
func Line(v uint) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEQ(FieldLine, v))
}

// PackageHintEQ applies the EQ predicate on the "PackageHint" field.
func PackageHintEQ(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEQ(FieldPackageHint, v))
}

// PackageHintNEQ applies the NEQ predicate on the "PackageHint" field.
func PackageHintNEQ(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldNEQ(FieldPackageHint, v))
}

// PackageHintIn applies the In predicate on the "PackageHint" field.
func PackageHintIn(vs ...string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldIn(FieldPackageHint, vs...))
}

// PackageHintNotIn applies the NotIn predicate on the "PackageHint" field.
func PackageHintNotIn(vs ...string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldNotIn(FieldPackageHint, vs...))
}

// PackageHintGT applies the GT predicate on the "PackageHint" field.
func PackageHintGT(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldGT(FieldPackageHint, v))
}

// PackageHintGTE applies the GTE predicate on the "PackageHint" field.
func PackageHintGTE(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldGTE(FieldPackageHint, v))
}

// PackageHintLT applies the LT predicate on the "PackageHint" field.
func PackageHintLT(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldLT(FieldPackageHint, v))
}

// PackageHintLTE applies the LTE predicate on the "PackageHint" field.
func PackageHintLTE(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldLTE(FieldPackageHint, v))
}

// PackageHintContains applies the Contains predicate on the "PackageHint" field.
func PackageHintContains(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldContains(FieldPackageHint, v))
}

// PackageHintHasPrefix applies the HasPrefix predicate on the "PackageHint" field.
func PackageHintHasPrefix(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldHasPrefix(FieldPackageHint, v))
}

// PackageHintHasSuffix applies the HasSuffix predicate on the "PackageHint" field.
func PackageHintHasSuffix(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldHasSuffix(FieldPackageHint, v))
}

// PackageHintIsNil applies the IsNil predicate on the "PackageHint" field.
func PackageHintIsNil() predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldIsNull(FieldPackageHint))
}

// PackageHintNotNil applies the NotNil predicate on the "PackageHint" field.
func PackageHintNotNil() predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldNotNull(FieldPackageHint))
}

// PackageHintEqualFold applies the EqualFold predicate on the "PackageHint" field.
func PackageHintEqualFold(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEqualFold(FieldPackageHint, v))
}

// PackageHintContainsFold applies the ContainsFold predicate on the "PackageHint" field.
func PackageHintContainsFold(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldContainsFold(FieldPackageHint, v))
}

// ModuleNameEQ applies the EQ predicate on the "ModuleName" field.
func ModuleNameEQ(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEQ(FieldModuleName, v))
}

// ModuleNameNEQ applies the NEQ predicate on the "ModuleName" field.
func ModuleNameNEQ(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldNEQ(FieldModuleName, v))
}

// ModuleNameIn applies the In predicate on the "ModuleName" field.
func ModuleNameIn(vs ...string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldIn(FieldModuleName, vs...))
}

// ModuleNameNotIn applies the NotIn predicate on the "ModuleName" field.
func ModuleNameNotIn(vs ...string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldNotIn(FieldModuleName, vs...))
}

// ModuleNameGT applies the GT predicate on the "ModuleName" field.
func ModuleNameGT(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldGT(FieldModuleName, v))
}

// ModuleNameGTE applies the GTE predicate on the "ModuleName" field.
func ModuleNameGTE(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldGTE(FieldModuleName, v))
}

// ModuleNameLT applies the LT predicate on the "ModuleName" field.
func ModuleNameLT(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldLT(FieldModuleName, v))
}

// ModuleNameLTE applies the LTE predicate on the "ModuleName" field.
func ModuleNameLTE(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldLTE(FieldModuleName, v))
}

// ModuleNameContains applies the Contains predicate on the "ModuleName" field.
func ModuleNameContains(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldContains(FieldModuleName, v))
}

// ModuleNameHasPrefix applies the HasPrefix predicate on the "ModuleName" field.
func ModuleNameHasPrefix(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldHasPrefix(FieldModuleName, v))
}

// ModuleNameHasSuffix applies the HasSuffix predicate on the "ModuleName" field.
func ModuleNameHasSuffix(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldHasSuffix(FieldModuleName, v))
}

// ModuleNameEqualFold applies the EqualFold predicate on the "ModuleName" field.
func ModuleNameEqualFold(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEqualFold(FieldModuleName, v))
}

// ModuleNameContainsFold applies the ContainsFold predicate on the "ModuleName" field.
func ModuleNameContainsFold(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldContainsFold(FieldModuleName, v))
}

// ModuleItemEQ applies the EQ predicate on the "ModuleItem" field.
func ModuleItemEQ(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEQ(FieldModuleItem, v))
}

// ModuleItemNEQ applies the NEQ predicate on the "ModuleItem" field.
func ModuleItemNEQ(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldNEQ(FieldModuleItem, v))
}

// ModuleItemIn applies the In predicate on the "ModuleItem" field.
func ModuleItemIn(vs ...string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldIn(FieldModuleItem, vs...))
}

// ModuleItemNotIn applies the NotIn predicate on the "ModuleItem" field.
func ModuleItemNotIn(vs ...string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldNotIn(FieldModuleItem, vs...))
}

// ModuleItemGT applies the GT predicate on the "ModuleItem" field.
func ModuleItemGT(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldGT(FieldModuleItem, v))
}

// ModuleItemGTE applies the GTE predicate on the "ModuleItem" field.
func ModuleItemGTE(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldGTE(FieldModuleItem, v))
}

// ModuleItemLT applies the LT predicate on the "ModuleItem" field.
func ModuleItemLT(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldLT(FieldModuleItem, v))
}

// ModuleItemLTE applies the LTE predicate on the "ModuleItem" field.
func ModuleItemLTE(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldLTE(FieldModuleItem, v))
}

// ModuleItemContains applies the Contains predicate on the "ModuleItem" field.
func ModuleItemContains(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldContains(FieldModuleItem, v))
}

// ModuleItemHasPrefix applies the HasPrefix predicate on the "ModuleItem" field.
func ModuleItemHasPrefix(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldHasPrefix(FieldModuleItem, v))
}

// ModuleItemHasSuffix applies the HasSuffix predicate on the "ModuleItem" field.
func ModuleItemHasSuffix(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldHasSuffix(FieldModuleItem, v))
}

// ModuleItemIsNil applies the IsNil predicate on the "ModuleItem" field.
func ModuleItemIsNil() predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldIsNull(FieldModuleItem))
}

// ModuleItemNotNil applies the NotNil predicate on the "ModuleItem" field.
func ModuleItemNotNil() predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldNotNull(FieldModuleItem))
}

// ModuleItemEqualFold applies the EqualFold predicate on the "ModuleItem" field.
func ModuleItemEqualFold(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEqualFold(FieldModuleItem, v))
}

// ModuleItemContainsFold applies the ContainsFold predicate on the "ModuleItem" field.
func ModuleItemContainsFold(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldContainsFold(FieldModuleItem, v))
}

// ModuleAliasEQ applies the EQ predicate on the "ModuleAlias" field.
func ModuleAliasEQ(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEQ(FieldModuleAlias, v))
}

// ModuleAliasNEQ applies the NEQ predicate on the "ModuleAlias" field.
func ModuleAliasNEQ(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldNEQ(FieldModuleAlias, v))
}

// ModuleAliasIn applies the In predicate on the "ModuleAlias" field.
func ModuleAliasIn(vs ...string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldIn(FieldModuleAlias, vs...))
}

// ModuleAliasNotIn applies the NotIn predicate on the "ModuleAlias" field.
func ModuleAliasNotIn(vs ...string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldNotIn(FieldModuleAlias, vs...))
}

// ModuleAliasGT applies the GT predicate on the "ModuleAlias" field.
func ModuleAliasGT(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldGT(FieldModuleAlias, v))
}

// ModuleAliasGTE applies the GTE predicate on the "ModuleAlias" field.
func ModuleAliasGTE(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldGTE(FieldModuleAlias, v))
}

// ModuleAliasLT applies the LT predicate on the "ModuleAlias" field.
func ModuleAliasLT(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldLT(FieldModuleAlias, v))
}

// ModuleAliasLTE applies the LTE predicate on the "ModuleAlias" field.
func ModuleAliasLTE(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldLTE(FieldModuleAlias, v))
}

// ModuleAliasContains applies the Contains predicate on the "ModuleAlias" field.
func ModuleAliasContains(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldContains(FieldModuleAlias, v))
}

// ModuleAliasHasPrefix applies the HasPrefix predicate on the "ModuleAlias" field.
func ModuleAliasHasPrefix(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldHasPrefix(FieldModuleAlias, v))
}

// ModuleAliasHasSuffix applies the HasSuffix predicate on the "ModuleAlias" field.
func ModuleAliasHasSuffix(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldHasSuffix(FieldModuleAlias, v))
}

// ModuleAliasIsNil applies the IsNil predicate on the "ModuleAlias" field.
func ModuleAliasIsNil() predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldIsNull(FieldModuleAlias))
}

// ModuleAliasNotNil applies the NotNil predicate on the "ModuleAlias" field.
func ModuleAliasNotNil() predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldNotNull(FieldModuleAlias))
}

// ModuleAliasEqualFold applies the EqualFold predicate on the "ModuleAlias" field.
func ModuleAliasEqualFold(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEqualFold(FieldModuleAlias, v))
}

// ModuleAliasContainsFold applies the ContainsFold predicate on the "ModuleAlias" field.
func ModuleAliasContainsFold(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldContainsFold(FieldModuleAlias, v))
}

// IsWildCardUsageEQ applies the EQ predicate on the "IsWildCardUsage" field.
func IsWildCardUsageEQ(v bool) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEQ(FieldIsWildCardUsage, v))
}

// IsWildCardUsageNEQ applies the NEQ predicate on the "IsWildCardUsage" field.
func IsWildCardUsageNEQ(v bool) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldNEQ(FieldIsWildCardUsage, v))
}

// IsWildCardUsageIsNil applies the IsNil predicate on the "IsWildCardUsage" field.
func IsWildCardUsageIsNil() predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldIsNull(FieldIsWildCardUsage))
}

// IsWildCardUsageNotNil applies the NotNil predicate on the "IsWildCardUsage" field.
func IsWildCardUsageNotNil() predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldNotNull(FieldIsWildCardUsage))
}

// IdentifierEQ applies the EQ predicate on the "Identifier" field.
func IdentifierEQ(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEQ(FieldIdentifier, v))
}

// IdentifierNEQ applies the NEQ predicate on the "Identifier" field.
func IdentifierNEQ(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldNEQ(FieldIdentifier, v))
}

// IdentifierIn applies the In predicate on the "Identifier" field.
func IdentifierIn(vs ...string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldIn(FieldIdentifier, vs...))
}

// IdentifierNotIn applies the NotIn predicate on the "Identifier" field.
func IdentifierNotIn(vs ...string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldNotIn(FieldIdentifier, vs...))
}

// IdentifierGT applies the GT predicate on the "Identifier" field.
func IdentifierGT(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldGT(FieldIdentifier, v))
}

// IdentifierGTE applies the GTE predicate on the "Identifier" field.
func IdentifierGTE(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldGTE(FieldIdentifier, v))
}

// IdentifierLT applies the LT predicate on the "Identifier" field.
func IdentifierLT(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldLT(FieldIdentifier, v))
}

// IdentifierLTE applies the LTE predicate on the "Identifier" field.
func IdentifierLTE(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldLTE(FieldIdentifier, v))
}

// IdentifierContains applies the Contains predicate on the "Identifier" field.
func IdentifierContains(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldContains(FieldIdentifier, v))
}

// IdentifierHasPrefix applies the HasPrefix predicate on the "Identifier" field.
func IdentifierHasPrefix(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldHasPrefix(FieldIdentifier, v))
}

// IdentifierHasSuffix applies the HasSuffix predicate on the "Identifier" field.
func IdentifierHasSuffix(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldHasSuffix(FieldIdentifier, v))
}

// IdentifierIsNil applies the IsNil predicate on the "Identifier" field.
func IdentifierIsNil() predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldIsNull(FieldIdentifier))
}

// IdentifierNotNil applies the NotNil predicate on the "Identifier" field.
func IdentifierNotNil() predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldNotNull(FieldIdentifier))
}

// IdentifierEqualFold applies the EqualFold predicate on the "Identifier" field.
func IdentifierEqualFold(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEqualFold(FieldIdentifier, v))
}

// IdentifierContainsFold applies the ContainsFold predicate on the "Identifier" field.
func IdentifierContainsFold(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldContainsFold(FieldIdentifier, v))
}

// UsageFilePathEQ applies the EQ predicate on the "UsageFilePath" field.
func UsageFilePathEQ(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEQ(FieldUsageFilePath, v))
}

// UsageFilePathNEQ applies the NEQ predicate on the "UsageFilePath" field.
func UsageFilePathNEQ(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldNEQ(FieldUsageFilePath, v))
}

// UsageFilePathIn applies the In predicate on the "UsageFilePath" field.
func UsageFilePathIn(vs ...string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldIn(FieldUsageFilePath, vs...))
}

// UsageFilePathNotIn applies the NotIn predicate on the "UsageFilePath" field.
func UsageFilePathNotIn(vs ...string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldNotIn(FieldUsageFilePath, vs...))
}

// UsageFilePathGT applies the GT predicate on the "UsageFilePath" field.
func UsageFilePathGT(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldGT(FieldUsageFilePath, v))
}

// UsageFilePathGTE applies the GTE predicate on the "UsageFilePath" field.
func UsageFilePathGTE(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldGTE(FieldUsageFilePath, v))
}

// UsageFilePathLT applies the LT predicate on the "UsageFilePath" field.
func UsageFilePathLT(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldLT(FieldUsageFilePath, v))
}

// UsageFilePathLTE applies the LTE predicate on the "UsageFilePath" field.
func UsageFilePathLTE(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldLTE(FieldUsageFilePath, v))
}

// UsageFilePathContains applies the Contains predicate on the "UsageFilePath" field.
func UsageFilePathContains(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldContains(FieldUsageFilePath, v))
}

// UsageFilePathHasPrefix applies the HasPrefix predicate on the "UsageFilePath" field.
func UsageFilePathHasPrefix(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldHasPrefix(FieldUsageFilePath, v))
}

// UsageFilePathHasSuffix applies the HasSuffix predicate on the "UsageFilePath" field.
func UsageFilePathHasSuffix(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldHasSuffix(FieldUsageFilePath, v))
}

// UsageFilePathEqualFold applies the EqualFold predicate on the "UsageFilePath" field.
func UsageFilePathEqualFold(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEqualFold(FieldUsageFilePath, v))
}

// UsageFilePathContainsFold applies the ContainsFold predicate on the "UsageFilePath" field.
func UsageFilePathContainsFold(v string) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldContainsFold(FieldUsageFilePath, v))
}

// LineEQ applies the EQ predicate on the "Line" field.
func LineEQ(v uint) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldEQ(FieldLine, v))
}

// LineNEQ applies the NEQ predicate on the "Line" field.
func LineNEQ(v uint) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldNEQ(FieldLine, v))
}

// LineIn applies the In predicate on the "Line" field.
func LineIn(vs ...uint) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldIn(FieldLine, vs...))
}

// LineNotIn applies the NotIn predicate on the "Line" field.
func LineNotIn(vs ...uint) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldNotIn(FieldLine, vs...))
}

// LineGT applies the GT predicate on the "Line" field.
func LineGT(v uint) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldGT(FieldLine, v))
}

// LineGTE applies the GTE predicate on the "Line" field.
func LineGTE(v uint) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldGTE(FieldLine, v))
}

// LineLT applies the LT predicate on the "Line" field.
func LineLT(v uint) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldLT(FieldLine, v))
}

// LineLTE applies the LTE predicate on the "Line" field.
func LineLTE(v uint) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.FieldLTE(FieldLine, v))
}

// HasCodeFile applies the HasEdge predicate on the "code_file" edge.
func HasCodeFile() predicate.UsageEvidence {
	return predicate.UsageEvidence(func(s *sql.Selector) {
		step := sqlgraph.NewStep(
			sqlgraph.From(Table, FieldID),
			sqlgraph.Edge(sqlgraph.M2O, false, CodeFileTable, CodeFileColumn),
		)
		sqlgraph.HasNeighbors(s, step)
	})
}

// HasCodeFileWith applies the HasEdge predicate on the "code_file" edge with a given conditions (other predicates).
func HasCodeFileWith(preds ...predicate.CodeFile) predicate.UsageEvidence {
	return predicate.UsageEvidence(func(s *sql.Selector) {
		step := newCodeFileStep()
		sqlgraph.HasNeighborsWith(s, step, func(s *sql.Selector) {
			for _, p := range preds {
				p(s)
			}
		})
	})
}

// And groups predicates with the AND operator between them.
func And(predicates ...predicate.UsageEvidence) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.AndPredicates(predicates...))
}

// Or groups predicates with the OR operator between them.
func Or(predicates ...predicate.UsageEvidence) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.OrPredicates(predicates...))
}

// Not applies the not operator on the given predicate.
func Not(p predicate.UsageEvidence) predicate.UsageEvidence {
	return predicate.UsageEvidence(sql.NotPredicates(p))
}
