import json
import argparse
from collections import defaultdict


def load_judge_results(input_file):
    """Load judge results from JSONL file."""
    results = []
    overall_stats = None

    with open(input_file, "r") as f:
        for line_num, line in enumerate(f):
            data = json.loads(line.strip())
            if line_num == 0 and "averages" in data:
                overall_stats = data
            else:
                results.append(data)

    return results, overall_stats


def compute_group_statistics(results, group_by_field):
    """Compute statistics grouped by specified field."""
    groups = defaultdict(list)

    # Group results
    for result in results:
        if group_by_field in result:
            group_key = result[group_by_field]
            groups[group_key].append(result)

    # Compute statistics for each group
    group_stats = {}
    semantic_categories = ["names_orgs", "monetary", "dates", "json_format"]

    for group_key, group_results in groups.items():
        if not group_results:
            continue

        original_scores = [r["original_score"] for r in group_results]

        group_stats[group_key] = {
            "sample_count": len(group_results),
            "original_avg": sum(original_scores) / len(original_scores),
            "original_min": min(original_scores),
            "original_max": max(original_scores),
            "semantic_avgs": {
                cat: sum(r["semantic_scores"][cat] for r in group_results)
                / len(group_results)
                for cat in semantic_categories
            },
        }

    return group_stats


def print_statistics(group_stats, group_name):
    """Print formatted statistics."""
    print(f"\n=== Statistics by {group_name} ===")

    for group_key in sorted(group_stats.keys()):
        stats = group_stats[group_key]
        print(f"\n{group_name.title()} {group_key}:")
        print(f"  Sample count: {stats['sample_count']}")
        print(
            f"  Original score: {stats['original_avg']:.2f} (min: {stats['original_min']:.1f}, max: {stats['original_max']:.1f})"
        )
        print("  Semantic scores:")
        for cat, score in stats["semantic_avgs"].items():
            print(f"    {cat}: {score:.3f}")


def save_statistics(group_stats, output_file, group_name):
    """Save statistics to JSON file."""
    output_data = {"group_by": group_name, "statistics": group_stats}

    with open(output_file, "w") as f:
        json.dump(output_data, f, indent=2)

    print(f"\nStatistics saved to {output_file}")


def main():
    parser = argparse.ArgumentParser(
        description="Compute summary statistics from judge results"
    )
    parser.add_argument("input_file", help="Input JSONL file with judge results")
    parser.add_argument(
        "--group-by", default="turns", help="Field to group by (default: turns)"
    )
    parser.add_argument("--output", help="Output JSON file for statistics")
    parser.add_argument(
        "--print-only", action="store_true", help="Only print statistics, don't save"
    )

    args = parser.parse_args()

    print(f"Loading judge results from {args.input_file}")
    results, overall_stats = load_judge_results(args.input_file)

    if overall_stats:
        print("\nOverall statistics:")
        print(f"  Total samples: {overall_stats.get('total_samples', len(results))}")
        if "averages" in overall_stats:
            avg = overall_stats["averages"]
            print(f"  Overall original avg: {avg['original_avg']:.2f}")
            print(f"  Overall semantic avgs: {avg['semantic_avgs']}")

    # Compute grouped statistics
    group_stats = compute_group_statistics(results, args.group_by)

    if group_stats:
        print_statistics(group_stats, args.group_by)

        if not args.print_only:
            output_file = (
                args.output
                or f"{args.input_file.replace('.jsonl', '')}_stats_by_{args.group_by}.json"
            )
            save_statistics(group_stats, output_file, args.group_by)
    else:
        print(f"\nNo results found with field '{args.group_by}'")


if __name__ == "__main__":
    main()
