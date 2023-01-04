# User defined gitlint commit rule that forbids fixup commits.
#
# Suggested by gitlint author in
# https://github.com/jorisroovers/gitlint/issues/139#issuecomment-674160001.

from gitlint.rules import CommitRule, RuleViolation

class DisallowFixup(CommitRule):
    name = "fixup-disallowed"
    # A rule MUST have a *unique* id, we recommend starting with UC (for User-defined Commit-rule).
    id = "UC1"

    def validate(self, commit):
        if commit.is_fixup_commit:
            return [RuleViolation(self.id, "Fixup commits are not allowed", line_nr=1)]
