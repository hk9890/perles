---
name: "Mediated Investigation"
description: "High-quality investigation with devil's advocate, counter-investigation, parallel review, and external validation"
category: "Analysis"
workers: 6
target_mode: "orchestration"
---

# Mediated Investigation Workflow

## Overview

A rigorous 6-worker investigation workflow designed to produce the highest quality investigation possible. The workflow includes adversarial validation (devil's advocate, counter-investigation), parallel sub-agent reviews, and external validation to ensure conclusions are rock-solid before creating an implementation plan.

**Philosophy:** Quality over speed. Every conclusion is challenged, every alternative is explored, every reference is verified.

**Flow:**
```
Phase 1:  Worker 1 (Mediator)           → Create outline + hypothesis list
Phase 2A: Worker 2 (Researcher)         → Broad exploration with confidence scores
Phase 2B: Worker 2 (Researcher)         → Deep dive on high-confidence areas
Phase 3:  Worker 3 (Devil's Advocate)   → Challenge findings, question assumptions
Phase 4:  Worker 4 (Counter-Researcher) → Try to prove conclusion is WRONG
Phase 5:  Worker 5 (Reviewer)           → Parallel sub-agent review (3 dimensions)
Phase 6:  Worker 6 (External Validator) → Fresh eyes - can someone else understand?
Phase 7:  Worker 1 (Mediator)           → Synthesize into implementation plan
```

**Output:**
- `docs/investigations/YYYY-MM-DD-HHMM-{name}-outline.md` - Investigation with findings
- `docs/investigations/YYYY-MM-DD-HHMM-{name}-plan.md` - Implementation plan

**Quality Gates:**
| Gate | Pass Condition | Failure Action |
|------|----------------|----------------|
| Devil's Advocate | All challenges addressed | Researcher revises |
| Counter-Investigation | Cannot prove alternative | Back to research |
| Review Sub-Agents | All 3 approve | Address specific issues |
| External Validation | Fresh worker understands | Mediator fixes docs |

---

## Roles

### Worker 1: Mediator
**Goal:** Lead the investigation - create structure, fix validation issues, synthesize results.

**Responsibilities:**
- Create investigation outline with hypothesis list and confidence template
- Fix documentation if external validation fails
- Synthesize all findings into actionable implementation plan
- Ensure investigation stays focused on the original problem

**Phases Active:** 1, 7 (and fixes in Phase 6 if needed)

---

### Worker 2: Researcher
**Goal:** Explore the codebase in two phases - broad then deep.

**Responsibilities:**
- Phase 2A: Broad exploration to map the landscape
- Phase 2B: Deep dive into high-confidence areas
- Assign confidence scores (High/Medium/Low) to each finding
- Use CONFIRMED/RULED OUT annotations for hypotheses
- Address challenges from Devil's Advocate

**Output:**
- Confidence-scored findings
- Code references with file:line citations
- CRITICAL FINDING callouts
- Code reference tables

**Phases Active:** 2A, 2B (and revisions after Phase 3)

---

### Worker 3: Devil's Advocate
**Goal:** Poke holes in the researcher's findings to strengthen the investigation.

**Responsibilities:**
- Question assumptions - "Did you verify this, or just assume?"
- Challenge weak evidence - "This conclusion needs more support"
- Identify gaps - "What about scenario X?"
- Force researcher to strengthen weak points

**Output:** List of challenges that must be addressed before proceeding.

**Phases Active:** 3

---

### Worker 4: Counter-Researcher
**Goal:** Actively try to prove the researcher's conclusion is WRONG.

**Responsibilities:**
- Identify alternative hypotheses
- Research evidence that supports alternatives
- Try to prove ANY alternative explanation
- No time limit - thoroughness over speed

**Output:**
- If succeeds: Alternative explanation with evidence → Back to research
- If fails: "Could not disprove conclusion" → High confidence

**Phases Active:** 4

---

### Worker 5: Reviewer (with Sub-Agents)
**Goal:** Comprehensive review using parallel sub-agents for different dimensions.

**Responsibilities:**
- Spawn 3 sub-agents to review in parallel:
  1. **Code Accuracy Agent**: Verify every file:line reference exists
  2. **Completeness Agent**: Check all outline questions were answered
  3. **Logic Agent**: Validate conclusions follow from evidence
- Synthesize sub-agent findings into final review verdict

**Output:** APPROVED or NEEDS WORK with specific issues from each dimension.

**Phases Active:** 5

---

### Worker 6: External Validator
**Goal:** Fresh perspective - test if documentation is clear enough for someone else.

**Responsibilities:**
- Has NOT seen any of the investigation process
- Reads ONLY the final documents
- Answers: "Can I understand the problem and proposed solution?"
- Identifies confusing or unclear sections

**Output:**
- VALIDATED: Documentation is clear and actionable
- NEEDS CLARIFICATION: Specific sections that are unclear

**Phases Active:** 6

---

## Workflow Phases

### Phase 1: Create Outline (Mediator)

**Coordinator assigns Worker 1 with prompt:**
```
You are the **Mediator** for a high-quality investigation workflow.

**Problem to Investigate:**
[Describe the problem or question]

**Your Task:**
1. Understand the problem statement
2. Create an investigation outline at: `docs/investigations/YYYY-MM-DD-HHMM-{name}-outline.md`

**Outline Structure:**
```markdown
# Investigation: {Problem Title}

## Problem Statement

{2-3 paragraphs describing the problem and why it matters}

## Initial Hypotheses

List possible explanations to investigate:
- [ ] **H1:** {First hypothesis} - Confidence: TBD
- [ ] **H2:** {Second hypothesis} - Confidence: TBD
- [ ] **H3:** {Third hypothesis} - Confidence: TBD

---

## 1. {First Area to Investigate}

### Questions to Answer
- {Specific question}
- {Specific question}

### Files/Functions to Examine
- [ ] {Suggested file or pattern} (`path/to/file.go`)
- [ ] {Suggested file or pattern}

### Findings
[To be filled by Researcher]

### Confidence Score
[To be filled by Researcher: High/Medium/Low with rationale]

---

## 2. {Second Area to Investigate}
[Same structure...]

---

## N. Gap Analysis / Root Cause

### Questions to Answer
- At what point do the two paths diverge?
- Is the data available but not used correctly?
- What's the root cause of the issue?

### Comparison Points
- [ ] Compare X between the two paths
- [ ] Identify any hardcoded assumptions

### Findings
[To be filled by Researcher]

---

## Key Files and Functions Summary

### Primary Files
| File | Purpose | Examined |
|------|---------|----------|
| `path/to/file.go` | {purpose} | [ ] |

### Key Functions
| Function | File | Purpose |
|----------|------|---------|
| {name} | {file} | {purpose} |

---

## Investigation Output

### Hypothesis Status
| ID | Hypothesis | Status | Confidence | Evidence |
|----|------------|--------|------------|----------|
| H1 | {hypothesis} | TBD | TBD | TBD |

### Root Cause
{To be filled after research}

### Code References
| File | Line(s) | Function/Purpose |
|------|---------|------------------|
| `path/to/file.go` | 123-145 | {description} |

### Solution Options
**OPTION A: {Description}**
- {Details}
- Confidence: {High/Medium/Low}

**OPTION B: {Description}** (if applicable)
- {Details}
- Confidence: {High/Medium/Low}

---

## Challenge Log
[To be filled by Devil's Advocate and Researcher responses]

## Counter-Investigation Log
[To be filled by Counter-Researcher]

## Validation Status
- [ ] Devil's Advocate challenges addressed
- [ ] Counter-investigation passed
- [ ] Review sub-agents approved
- [ ] External validation passed
```

**Critical Rules:**
- Create STRUCTURE only - do NOT fill in findings
- Include hypothesis list with confidence template
- Define clear, answerable questions
- Leave space for Challenge Log and Counter-Investigation Log
```

**Coordinator:** Wait for completion, then proceed to Phase 2A.

---

### Phase 2A: Broad Research (Researcher)

**Coordinator assigns Worker 2 with prompt:**
```
You are the **Researcher** for a high-quality investigation workflow.

**Investigation Outline:** `docs/investigations/{filename}-outline.md`

**Phase 2A Task: BROAD EXPLORATION**

Your goal is to map the landscape before going deep. For each section:

1. Read the questions and suggested files
2. Explore broadly - understand the overall structure
3. Assign confidence scores to each finding:
   - **High**: Verified with code evidence, multiple references
   - **Medium**: Likely correct but needs deeper verification
   - **Low**: Uncertain, needs more investigation

**Requirements:**
- Use Grep/Glob/Read to explore
- Cite file:line references
- Mark hypotheses as LIKELY/UNLIKELY based on initial exploration
- Identify which areas need deep diving in Phase 2B
- DO NOT go deep yet - breadth first

**Output for each section:**
- Initial findings with confidence scores
- Areas flagged for deep dive
- Updated hypothesis confidence levels

Update the outline file with your broad findings. Mark each finding with confidence.
```

**Coordinator:** Wait for completion, then proceed to Phase 2B.

---

### Phase 2B: Deep Dive Research (Researcher)

**Coordinator assigns Worker 2 with prompt:**
```
You are the **Researcher** continuing the investigation.

**Investigation Outline:** `docs/investigations/{filename}-outline.md`

**Phase 2B Task: DEEP DIVE**

Based on your Phase 2A findings, go deep on high-confidence areas:

1. Focus on Medium and High confidence hypotheses
2. Verify assumptions with concrete code evidence
3. Mark hypotheses as CONFIRMED or RULED OUT
4. Use these callout patterns:
   - **CRITICAL FINDING:** for important discoveries
   - **CONFIRMED:** hypothesis verified with evidence
   - **RULED OUT:** hypothesis disproven with evidence

**Requirements:**
- Include code snippets for key findings
- Create summary tables for code references
- Answer ALL questions in the outline
- Note unexpected discoveries
- Complete the "Investigation Output" section

**Your findings will be challenged by a Devil's Advocate, so:**
- Be thorough - weak findings will be questioned
- Distinguish between "verified" and "assumed"
- Acknowledge uncertainty where it exists
```

**Coordinator:** Wait for completion, then proceed to Phase 3.

---

### Phase 3: Challenge (Devil's Advocate)

**Coordinator assigns Worker 3 with prompt:**
```
You are the **Devil's Advocate** for this investigation.

**Investigation Document:** `docs/investigations/{filename}-outline.md`

**Your Task:** Challenge the researcher's findings to make them stronger.

For each finding, ask yourself:
1. **Verification**: Was this verified with code, or just assumed?
2. **Alternatives**: Could there be another explanation?
3. **Evidence**: Is the evidence strong enough to support the conclusion?
4. **Gaps**: What wasn't investigated that should have been?
5. **Assumptions**: What hidden assumptions are being made?

**Challenge Format:**
```markdown
## Challenge Log

### Challenge 1: {Finding/Section}
**Type:** [Verification/Alternative/Evidence/Gap/Assumption]
**Challenge:** {Your challenge question}
**Severity:** [Must Address/Should Address/Consider]

### Challenge 2: ...
```

**Rules:**
- Be rigorous but constructive
- "Must Address" challenges block progress until resolved
- Focus on strengthening the investigation, not blocking it
- Acknowledge strong findings - not everything needs challenging

Add your challenges to the "Challenge Log" section of the document.
```

**Coordinator:** Send challenges to Researcher (Worker 2) for response.

**Coordinator assigns Worker 2 with prompt:**
```
The Devil's Advocate has challenged your findings.

**Investigation Document:** `docs/investigations/{filename}-outline.md`

Review the Challenge Log and address each challenge:
- For "Must Address": Provide additional evidence or revise finding
- For "Should Address": Explain your reasoning or provide clarification
- For "Consider": Acknowledge and note if relevant

Update the Challenge Log with your responses:
```markdown
### Challenge 1: {Finding/Section}
**Challenge:** {original challenge}
**Response:** {your response with evidence}
**Status:** [Resolved/Acknowledged/Revised]
```

All "Must Address" challenges must be resolved before proceeding.
```

**Coordinator:** Verify all "Must Address" challenges are resolved. If not, loop. Then proceed to Phase 4.

---

### Phase 4: Counter-Investigation (Counter-Researcher)

**Coordinator assigns Worker 4 with prompt:**
```
You are the **Counter-Researcher** for this investigation.

**Investigation Document:** `docs/investigations/{filename}-outline.md`

**Your Mission:** Try to prove the researcher's conclusion is WRONG.

The researcher concluded: [summarize main conclusion]

**Your Task:**
1. Identify alternative hypotheses that could explain the problem
2. Research evidence that supports these alternatives
3. Try to prove ANY alternative explanation is correct
4. No time limit - be thorough

**If you SUCCEED in proving an alternative:**
- Document the alternative explanation with evidence
- This sends the investigation back to Phase 2B
- The researcher must address your alternative

**If you FAIL to prove an alternative:**
- Document what you tried and why it didn't work
- This INCREASES confidence in the original conclusion
- Write: "Counter-investigation complete. Could not disprove conclusion."

**Add to Counter-Investigation Log:**
```markdown
## Counter-Investigation Log

### Alternative Hypothesis 1: {Description}
**Evidence Sought:** {What would prove this}
**Research Done:** {What you investigated}
**Result:** [Supported/Not Supported]
**Conclusion:** {Why this alternative does or doesn't hold}

### Final Verdict
[Could not disprove original conclusion / Found viable alternative]
```

Be thorough. A failed counter-investigation is valuable - it increases confidence.
```

**Coordinator:**
- If alternative proven → Send back to Worker 2 (Phase 2B) with new direction
- If no alternative proven → Proceed to Phase 5

---

### Phase 5: Review (Reviewer with Sub-Agents)

**Coordinator assigns Worker 5 with prompt:**
```
You are the **Reviewer** for this investigation.

**Investigation Document:** `docs/investigations/{filename}-outline.md`

**Your Task:** Conduct a comprehensive review using 3 parallel sub-agents.

Spawn 3 sub-agents with these specific tasks:

**Sub-Agent 1: Code Accuracy**
```
Review the investigation document and verify EVERY code reference:
- Does each file:line citation exist?
- Are the code snippets accurate?
- Do the line numbers match the described content?

Output: List of verified references and any errors found.
```

**Sub-Agent 2: Completeness**
```
Review the investigation document for completeness:
- Were all questions in the outline answered?
- Are all hypotheses marked CONFIRMED or RULED OUT?
- Is the root cause clearly identified?
- Are all sections filled in?

Output: Completeness checklist with any gaps found.
```

**Sub-Agent 3: Logic**
```
Review the investigation document for logical soundness:
- Do conclusions follow from the evidence?
- Are there logical leaps or unsupported claims?
- Is the confidence scoring appropriate?
- Does the recommended solution address the root cause?

Output: Logic assessment with any issues found.
```

**After sub-agents complete, synthesize into final verdict:**

```markdown
## Review Summary

### Code Accuracy (Sub-Agent 1)
**Status:** [PASS/ISSUES]
- References verified: X/Y
- Issues: {list any}

### Completeness (Sub-Agent 2)
**Status:** [PASS/ISSUES]
- Sections complete: X/Y
- Gaps: {list any}

### Logic (Sub-Agent 3)
**Status:** [PASS/ISSUES]
- Conclusions sound: [Yes/No]
- Issues: {list any}

### Final Verdict
**Status:** [APPROVED/NEEDS WORK]
**Issues to Address:** {if any}
```
```

**Coordinator:**
- If APPROVED → Proceed to Phase 6
- If NEEDS WORK → Send issues to Researcher (Worker 2) for fixes, then re-review

---

### Phase 6: External Validation (External Validator)

**Coordinator assigns Worker 6 with prompt:**
```
You are the **External Validator** for this investigation.

**IMPORTANT:** You have NOT seen any of the investigation process. You are a fresh set of eyes.

**Documents to Review:**
- `docs/investigations/{filename}-outline.md`

**Your Task:** Determine if someone unfamiliar with this investigation can understand:
1. What is the problem being investigated?
2. What was discovered?
3. What is the proposed solution?
4. Why is this the right solution?

**Evaluation Criteria:**
- Can you follow the logic from problem to solution?
- Are there sections that are confusing or unclear?
- Is there jargon or context that isn't explained?
- Could you explain this to someone else?

**Output:**
```markdown
## External Validation

### Understanding Check
- [ ] Problem statement is clear
- [ ] Root cause is clearly explained
- [ ] Solution approach makes sense
- [ ] Could explain to someone else

### Clarity Issues
{List any sections that were confusing}

### Missing Context
{List any assumed knowledge that should be explained}

### Verdict
**Status:** [VALIDATED/NEEDS CLARIFICATION]
**Specific Issues:** {if any}
```

Be honest - if something is unclear, say so. This improves the documentation.
```

**Coordinator:**
- If VALIDATED → Proceed to Phase 7
- If NEEDS CLARIFICATION → Send issues to Mediator (Worker 1) for fixes, then re-validate

---

### Phase 7: Implementation Plan (Mediator)

**Coordinator assigns Worker 1 with prompt:**
```
You are the **Mediator** completing the investigation.

**Investigation Document:** `docs/investigations/{filename}-outline.md`

**Validation Status:**
- Devil's Advocate: Challenges addressed
- Counter-Investigation: Could not disprove conclusion
- Review: All sub-agents approved
- External Validation: Passed

**Your Task:** Create the implementation plan at `docs/investigations/{filename}-plan.md`

**Plan Structure:**
```markdown
# Implementation Plan: {Problem Title}

## Investigation Reference

See: `docs/investigations/{filename}-outline.md`

## Executive Summary

{2-3 sentences: problem, root cause, solution}

## Root Cause Analysis

### Summary of Findings
- {Key finding 1 - CONFIRMED}
- {Key finding 2 - CONFIRMED}
- {Alternatives ruled out by counter-investigation}

### Root Cause
{Clear statement of the root cause with confidence level}

### Validation Status
- [x] Devil's Advocate challenges addressed
- [x] Counter-investigation passed (could not disprove)
- [x] Review sub-agents approved
- [x] External validation passed

---

## Proposed Solution

### Approach
{Solution strategy with rationale}

### Files to Modify
| File | Changes | Risk |
|------|---------|------|
| `path/to/file.go` | {changes} | Low/Med/High |

---

## Implementation Steps

### Step 1: {Title}
{Description with code example if helpful}

**Verification:** {How to verify this step worked}

### Step 2: {Title}
...

---

## Testing Strategy

### Unit Tests
- [ ] `TestName_Scenario` - {description}

### Integration Tests
- [ ] `TestName_Flow` - {description}

### Manual Verification
- [ ] {Step to manually verify}

---

## Risks and Mitigations

### Risk 1: {Title}
{Description}
**Mitigation:** {How to address}

---

## Estimated Complexity

**Scope:** {Small/Medium/Large}
- Test additions: ~{X} lines
- Production changes: ~{Y} lines
- Confidence: {High - validated through counter-investigation}

---

## Success Criteria

1. {Specific criterion}
2. {Specific criterion}
3. All tests pass
4. No regressions
```

This plan has been validated through adversarial review. High confidence in approach.
```

**Coordinator:** Wait for completion, then signal workflow complete:
```
signal_workflow_complete(
    status="success",
    summary="Completed high-quality investigation for '{problem-title}'. All quality gates passed: devil's advocate, counter-investigation, review sub-agents, external validation. Implementation plan at docs/investigations/{filename}-plan.md"
)
```

---

## Coordinator Instructions

### Setup
```
1. Get problem description from user
2. Spawn 6 workers:
   - Worker 1: Mediator
   - Worker 2: Researcher
   - Worker 3: Devil's Advocate
   - Worker 4: Counter-Researcher
   - Worker 5: Reviewer
   - Worker 6: External Validator
3. Generate filename: YYYY-MM-DD-HHMM-{descriptive-name}
```

### Execution
```
Phase 1:  Worker 1 (Mediator)           → Create outline
Phase 2A: Worker 2 (Researcher)         → Broad research
Phase 2B: Worker 2 (Researcher)         → Deep dive

Phase 3:  Worker 3 (Devil's Advocate)   → Challenge findings
          Worker 2 (Researcher)         → Address challenges
          [Loop until all "Must Address" resolved]

Phase 4:  Worker 4 (Counter-Researcher) → Try to disprove
          [If alternative found → back to Phase 2B]
          [If no alternative → continue]

Phase 5:  Worker 5 (Reviewer)           → Sub-agent review
          [If NEEDS WORK → Worker 2 fixes → re-review]

Phase 6:  Worker 6 (External Validator) → Fresh eyes validation
          [If NEEDS CLARIFICATION → Worker 1 fixes → re-validate]

Phase 7:  Worker 1 (Mediator)           → Implementation plan
```

### Quality Gate Enforcement
```
DO NOT proceed past a quality gate until it passes:

Phase 3 → 4: All "Must Address" challenges resolved
Phase 4 → 5: Counter-investigation failed to find alternative
Phase 5 → 6: All 3 sub-agents approved
Phase 6 → 7: External validator understood documentation
```

---

## Common Pitfalls

1. **Rushing past challenges** - Every "Must Address" challenge must be resolved
2. **Weak counter-investigation** - Counter-researcher should genuinely try to disprove
3. **Skipping sub-agents** - All 3 review dimensions matter
4. **Fresh eyes contamination** - External validator must NOT have seen prior work
5. **Mediator doing research** - Outline phase is STRUCTURE only
6. **Low confidence findings treated as high** - Confidence scores must be accurate

---

## Success Criteria

A successful investigation produces:

- [ ] All hypotheses marked CONFIRMED or RULED OUT with evidence
- [ ] Devil's Advocate challenges addressed
- [ ] Counter-investigation could not disprove conclusion
- [ ] All 3 review sub-agents approved
- [ ] External validator understood the documentation
- [ ] Implementation plan based on validated findings
- [ ] High confidence in proposed solution

---

## Example Session

```
[User Request]
User: "The diffviewer hunks only work for working directory changes"

[Phase 1: Outline]
Worker 1 (Mediator): Creates outline with 3 hypotheses:
- H1: Parsing differs between working dir and commit diffs
- H2: State management issue in focus transitions
- H3: FileTree doesn't preserve hunk data

[Phase 2A: Broad Research]
Worker 2 (Researcher): Explores landscape
- H1: LOW confidence - both use same ParseDiff()
- H2: MEDIUM confidence - complex state machine
- H3: LOW confidence - pointers appear correct

[Phase 2B: Deep Dive]
Worker 2 (Researcher): Goes deep on H2
- RULED OUT: H1 and H3 with code evidence
- CRITICAL FINDING: Same parser used, hunks should work
- Conclusion: State management issue in getActiveFile()

[Phase 3: Challenge]
Worker 3 (Devil's Advocate):
- Challenge 1: "Did you verify FileTree with actual test, or just code inspection?"
- Challenge 2: "What about virtual scrolling mode?"
Worker 2: Addresses both with additional evidence

[Phase 4: Counter-Investigation]
Worker 4 (Counter-Researcher): Tries to prove H1 or H3
- Attempted to find parsing differences - none found
- Attempted to find pointer issues - none found
- Verdict: "Could not disprove conclusion"

[Phase 5: Review]
Worker 5 (Reviewer): Spawns 3 sub-agents
- Code Accuracy: 15/15 references verified
- Completeness: All questions answered
- Logic: Conclusions follow from evidence
- Verdict: APPROVED

[Phase 6: External Validation]
Worker 6 (External Validator): Fresh eyes
- Problem clear, solution clear
- Minor: "getActiveFile" needs more context
Worker 1: Adds explanation
Worker 6: VALIDATED

[Phase 7: Implementation Plan]
Worker 1 (Mediator): Creates plan
- High confidence (counter-investigation passed)
- Test-driven debugging approach
- ~150-200 lines test code, ~5-20 lines fix

[Complete]
Coordinator: "Investigation complete with high confidence. All quality gates passed."
```

---

## Learnings from Production Use

### What Worked Well

1. **Adversarial validation** - Devil's Advocate and Counter-Investigation catch blind spots
2. **Confidence scoring** - Forces researcher to distinguish verified from assumed
3. **Parallel sub-agent review** - Catches different types of issues efficiently
4. **External validation** - Ensures documentation is actually useful
5. **Quality gates** - Prevents premature conclusions

### Tips for Coordinators

1. **Don't rush quality gates** - Each gate exists for a reason
2. **Let counter-investigation be thorough** - A failed disproof is valuable
3. **External validator must be fresh** - Never contaminate with prior context
4. **Mediator synthesizes, doesn't copy** - Plan should distill, not duplicate

---

## Related Workflows

- **quick_plan.md** - When you know what to build (skip investigation)
- **research_proposal.md** - Multi-perspective design research
- **cook.md** - Executing tasks after investigation
- **debate.md** - When there are opposing technical positions
