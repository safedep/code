import { diff_match_patch } from "diff-match-patch";

// Convert stringified nested list to comma separated string for Filters
export function parseProbableStringifiedArray(str: string, maxLength: number) {
  try {
    const result = JSON.parse(str);
    if (Array.isArray(result)) {
      const parseableResultArray = result as string[][];
      return trimStringWithEllipsis(
        parseableResultArray.map((subEntry) => subEntry.join(",")).join(","),
        maxLength
      );
    }
    return trimStringWithEllipsis(str, maxLength);
  } catch (e) {
    return trimStringWithEllipsis(str, maxLength);
  }
}

// Trim string to specified length & add ellipsis if required
export function trimStringWithEllipsis(
  str: string,
  maxLengthInclusive: number
) {
  if (!str) return "";
  if (str.length > maxLengthInclusive) {
    return str.trimEnd().slice(0, maxLengthInclusive - 3) + "...";
  }
  return str.trimEnd();
}

// Removes symbols & extra spaces from given string.
export function normalizeDiffString(str: string) {
  return str
    .replace(/[.,/#!$%^&*;:[{}\]=\-_'"`~()]/g, "")
    .replace(/\s{2,}/g, " ")
    .trim();
}

// Note - This indicates, which section to show component and not added/removed
// It is useful when there exists a trivial diff, which has to be shown only in specific section but doesn't qualify as a diff
export enum BelongsIndicator {
  ORIGINAL = "original",
  MODIFIED = "modified",
  BOTH = "both",
}
export interface DiffComponents {
  belongsTo: BelongsIndicator;
  value: string;
  diffComponent?: "added" | "removed" | null;
}

const diffMatcher = new diff_match_patch();

// Parse diffs from diff_match_patch & filter out trivial diffs
// symbols, extra spaces newlines, etc need not be tracked for license diffs
export function diffStrings(sourceText: string, modifiedText: string) {
  const diffs = diffMatcher.diff_main(sourceText, modifiedText);
  const normalizedDiffs: DiffComponents[] = [];

  for (let i = 0; i < diffs.length; i++) {
    const currentDiff = diffs[i];
    const nextDiff = i + 1 < diffs.length ? diffs[i + 1] : null;

    if (
      currentDiff &&
      nextDiff &&
      currentDiff[0] === -1 &&
      nextDiff[0] === 1 &&
      normalizeDiffString(currentDiff[1]).toLowerCase() ===
        normalizeDiffString(nextDiff[1]).toLowerCase()
    ) {
      // Add both to their respective diff category without identifying as diffComponent
      normalizedDiffs.push({
        belongsTo: BelongsIndicator.ORIGINAL,
        value: currentDiff[1],
      });
      normalizedDiffs.push({
        belongsTo: BelongsIndicator.MODIFIED,
        value: nextDiff[1],
      });
      i++; // Skip next iteration, as its already handled
      continue;
    }

    normalizedDiffs.push({
      ...{
        diffComponent:
          currentDiff[0] === 1
            ? "added"
            : currentDiff[0] === -1
            ? "removed"
            : null,
      },
      belongsTo:
        currentDiff[0] === 1
          ? BelongsIndicator.MODIFIED
          : currentDiff[0] === -1
          ? BelongsIndicator.ORIGINAL
          : BelongsIndicator.BOTH,
      value: currentDiff[1],
    });
  }

  return normalizedDiffs;
}

// Based on presence of '\n' group the diffs corresponding to each line
export function splitDiffIntoLines(diffs: DiffComponents[]) {
  const lines: DiffComponents[][] = [[]];

  for (const diff of diffs) {
    const splitLines = diff.value.split("\n");

    // Handle \n at the beginning of diff
    let idx = 0;
    while (idx < splitLines.length && splitLines[idx].length === 0) {
      lines.push([]);
      idx++;
    }

    const subLines = splitLines.slice(idx);

    for (const subLine of subLines) {
      const isTrivialDiff = normalizeDiffString(subLine).length === 0;

      // Append to last line only if it is non-empty string
      if (subLine.length > 0) {
        if (isTrivialDiff) {
          lines[lines.length - 1].push({
            value: subLine,
            belongsTo: diff.belongsTo,
          });
        } else {
          lines[lines.length - 1].push({
            ...diff,
            value: subLine,
          });
        }

        // Create newline for intermittent newlines
        // (ignore last subLine, it is continued in next line)
        if (subLine != subLines[subLines.length - 1]) {
          lines.push([]);
        }
      }
    }
  }

  // console.log("Line splitter", {
  //   diffs,
  //   lines: lines.filter((diffLine) => diffLine.length > 0),
  // });

  // Filter out empty lines before returning;
  return lines.filter((diffLine) => diffLine.length > 0);
}
