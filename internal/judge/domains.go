package judge

import "github.com/koopa0/yomihon/internal/schema"

// domainTableRule is the rule id of a [domains] table that could not be used.
const domainTableRule RuleID = "schema.domain_table"

// checkDomains reports a [domains] table the rules file wrote and the reader
// could not use. Unlike the navigation roles it silences no other rule: the
// table only dresses a domain with a name and an endpaper, so the reader opens
// everything and every domain takes the endpaper its place in enums.domain
// gives it. What is lost is the author's own choice, which a run that said
// nothing about it would hide.
func checkDomains(domains schema.Domains) []Finding {
	if domains.Claim().Trustworthy() {
		return nil
	}
	return []Finding{domainTableFinding()}
}

// domainTableFinding is the whole of what is said about a refused table. It
// names no domain: a domain is the name of a folder, which may be withheld
// ground, and this finding goes to a program. The reason is in the log of
// yomihon serve, where the person who wrote the table reads it.
func domainTableFinding() Finding {
	return Finding{
		RuleID:          domainTableRule,
		Severity:        SeverityError,
		Path:            schema.ContractRelPath,
		Message:         "the [domains] table in the rules file could not be used, so no domain has the endpaper it asked for",
		Evidence:        "a table that pins a pattern that is not endpaper-01 to endpaper-12, pins one pattern for two domains, or names a domain that enums.domain does not list pins nothing; every domain then takes the endpaper its place in enums.domain gives it, and the names still apply. The domain is not quoted here, because it is the name of a folder",
		SuggestedAction: "correct the [domains] table in the rules file (yomihon serve logs the reason at startup), then judge the vault again",
		SourceRule:      sourceContractDomains,
		Fingerprint:     fingerprint(domainTableRule, schema.ContractRelPath, ""),
	}
}
