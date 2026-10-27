package audience

// Reachability (D-062). A customer is in the audience base when the filter
// matches and the account is active; it is reachable when it can receive at
// least one channel. The rule is added to every audience query here, so no
// client can leave it out or filter on it.
const (
	// SQLActive keeps only active accounts (not inactive, not deleted).
	SQLActive = "c.account_status = 'active'"
	// SQLEmailOK is true when an email can go out: address present, opted in,
	// not bounced.
	SQLEmailOK = "(c.email IS NOT NULL AND c.email_opt_in AND NOT c.email_bounced)"
	// SQLSMSOK is true when an SMS can go out: number present and opted in.
	SQLSMSOK = "(c.phone_e164 IS NOT NULL AND c.sms_opt_in)"
	// SQLReachable is true when at least one channel is available.
	SQLReachable = "(" + SQLEmailOK + " OR " + SQLSMSOK + ")"
)

// baseWhere is the compiled filter AND an active account.
func baseWhere(filterWhere string) string { return filterWhere + " AND " + SQLActive }

// previewSQL counts the base audience per language and channel combination.
// One row per language: both, email only, SMS only, unreachable.
func previewSQL(filterWhere string) string {
	return "SELECT c.language, " +
		"count(*) FILTER (WHERE " + SQLEmailOK + " AND " + SQLSMSOK + "), " +
		"count(*) FILTER (WHERE " + SQLEmailOK + " AND NOT " + SQLSMSOK + "), " +
		"count(*) FILTER (WHERE NOT " + SQLEmailOK + " AND " + SQLSMSOK + "), " +
		"count(*) FILTER (WHERE NOT " + SQLEmailOK + " AND NOT " + SQLSMSOK + ") " +
		"FROM dwh.dim_customer c WHERE " + baseWhere(filterWhere) + " " +
		"GROUP BY c.language ORDER BY c.language"
}

// recipientsSQL selects reachable customers ordered by customer_id. Contact
// details are returned only for an eligible channel. argsBefore is the number
// of filter arguments; the keyset cursor (when hasCursor) and the limit follow.
func recipientsSQL(filterWhere string, argsBefore int, hasCursor bool) string {
	sql := "SELECT c.customer_id, c.first_name, c.language, " +
		SQLEmailOK + ", " + SQLSMSOK + ", " +
		"CASE WHEN " + SQLEmailOK + " THEN c.email END, " +
		"CASE WHEN " + SQLSMSOK + " THEN c.phone_e164 END " +
		"FROM dwh.dim_customer c WHERE " + baseWhere(filterWhere) + " AND " + SQLReachable
	next := argsBefore + 1
	if hasCursor {
		sql += " AND " + rootKey + " > $" + itoa(next)
		next++
	}
	return sql + " ORDER BY " + rootKey + " LIMIT $" + itoa(next)
}
