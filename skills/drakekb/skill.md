---
name: drake-kb
description: >
  Search and read the Drake Software Knowledge Base for tax form documentation,
  screen-level field instructions, EF message explanations, and Drake Tax
  workflow guides. Use when preparing tax returns in Drake Tax, troubleshooting
  EF rejection messages, looking up how to enter data on a specific form or
  screen, or when the user asks about Drake software features. Also activate
  when the user mentions a specific IRS form number and needs to know how Drake
  handles it.
license: MIT
metadata:
  author: chonkskill
  version: "1.0"
  requires_tools:
    - drakekb:search
    - drakekb:read_article
    - drakekb:lookup_form
    - drakekb:index_stats
---

# Drake Software Knowledge Base

You have access to the full Drake Software Knowledge Base -- thousands of
articles covering every IRS form, Drake Tax screen, EF rejection message,
and workflow in the software. Use it to look up exactly how to complete a
form, what each field means, and how Drake handles specific tax situations.

The KB is searched on demand. You do not have the articles memorized. Always
search before answering Drake-specific questions.

---

## Quick Reference

| Task | Tool | Example |
|------|------|---------|
| Find articles by topic | `drakekb:search` | "dependent care credit" |
| Find articles by form | `drakekb:lookup_form` | form_number: "2441" |
| Read full article | `drakekb:read_article` | Pass the URL from search results |
| Check index size | `drakekb:index_stats` | No arguments needed |

---

## When to Search the KB

Search the Drake KB when:

- The user asks how to complete a specific form or schedule in Drake
- The user encounters an EF rejection message and needs the fix
- The user asks where to enter specific data in Drake Tax
- The user asks about a Drake Tax feature, setting, or workflow
- You need to know the exact screen, field, or data entry path for a form
- The user mentions a form number and you need Drake-specific instructions
- The user is troubleshooting a calculation, diagnostic, or reject code

Do not guess Drake-specific procedures from general tax knowledge. The KB
has the exact steps for Drake's software -- use it.

---

## Search Strategy

### Start broad, then narrow

1. Search with the form number or topic: `drakekb:search` with query "2441"
2. Review the titles and abstracts in the results
3. Pick the most relevant article and read it: `drakekb:read_article`
4. If the first article does not answer the question, try a more specific search

### Form-specific lookups

Use `drakekb:lookup_form` when the user mentions a specific IRS form. This
searches Drake-Tax articles specifically and tries multiple query patterns
to find the right article.

Common form lookups:
- "1040" -- main individual return
- "2441" -- child and dependent care credit
- "Schedule C" -- business income
- "W-2" -- wage entry
- "1099-NEC" -- nonemployee compensation
- "8812" -- child tax credit
- "Schedule A" -- itemized deductions

### EF message lookups

When the user encounters an EF rejection, search with the exact reject code:
- `drakekb:search` with query "EF message 0194"
- Or search with the reject description: "IND-031-04"

### Screen and field lookups

Drake organizes data entry by screens. When the user asks "where do I enter X":
1. Search for the topic: `drakekb:search` with query "where to enter [topic]"
2. Or search for the form: `drakekb:lookup_form` with the related form number
3. The article will specify the exact screen name and field

---

## Reading Articles

After finding a relevant article via search, use `drakekb:read_article` with
the URL from the search results to get the full content.

Articles typically contain:
- Step-by-step instructions for completing a form or screen
- Field-by-field descriptions
- Screenshots and navigation paths (described in text)
- Related articles and cross-references
- Notes about tax year-specific changes

When presenting information from an article:
- Cite the article title and URL so the user can visit it directly
- Quote the specific relevant section rather than summarizing loosely
- If the article references other articles, offer to look those up too

---

## Categories

The KB is organized into categories:

| Category | Content |
|----------|---------|
| Drake-Tax | Form instructions, screen guides, data entry, calculations |
| Resources | General tax reference, IRS guidelines, state-specific info |
| DAS | Drake Accounting Series (accounting software, not tax) |
| Banking | Bank products, refund transfers, EROs |
| Broadcasts | Software updates, announcements, version notes |
| Drake-Tax-Online | Drake Tax Online (cloud version) specific articles |
| Drake-Portals | Client portals, SecureFilePro |

For tax preparation, you will almost always want the **Drake-Tax** category.
Use the `category` parameter in search to filter: `category: "Drake-Tax"`.

---

## Workflow: Completing a Form

When the user needs to fill in a form in Drake:

1. Look up the form: `drakekb:lookup_form` with the form number
2. Read the main article for that form
3. Extract the screen name and navigation path
4. List the fields the user needs to fill in, with Drake's field names
5. Note any special entry methods (overrides, multi-form entries, linked screens)
6. Mention any common EF rejects associated with the form

### Example: User asks "How do I enter Form 2441 in Drake?"

1. `drakekb:lookup_form` with form_number "2441"
2. Find the main article (likely titled "Drake Tax - 2441 - Child and Dependent Care Credit")
3. `drakekb:read_article` with that URL
4. Present: which screen to open, what fields to complete, how dependent info links from the dependent screen, any special notes

---

## Workflow: Troubleshooting an EF Reject

1. Search for the reject code: `drakekb:search` with the exact code or message
2. Read the article explaining the reject
3. The article will explain:
   - What the reject means
   - What data is missing or incorrect
   - Which screen and field to check
   - How to fix the issue
4. Walk the user through the fix step by step

---

## Pitfalls

- **Do not guess Drake screen names or field locations.** Drake's UI changes
  between tax years and versions. Always look it up.

- **Do not confuse general IRS instructions with Drake instructions.** The IRS
  says what information is required. The KB says where Drake expects you to
  enter it. These are different questions.

- **Articles may be tax-year specific.** Drake updates its KB for each tax
  year. The current articles reflect the current version. If the user is working
  on a prior-year return, note that screen layouts may differ.

- **The search index loads on first use.** The first search in a session takes
  a few seconds to download the index. Subsequent searches are instant.

- **Some articles reference screenshots.** The text extraction captures the
  surrounding text but not the images. If the user needs visual guidance,
  suggest they visit the article URL directly.
