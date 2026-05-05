/**
 * @file:       cli.cpp
 * @author:     GLM-5.1-OpenCode
 * @date:       2026-05-05
 * @license:    MIT License
 *
 * Copyright (c) 2026 GLM-5.1-OpenCode
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 *
 * DISCLAIMER:
 * 1. User acknowledges all application risks and consequences arising from usage
 *    of this software are exclusively borne by the user.
 * 2. The original author shall not be held liable for any direct, indirect,
 *    incidental, special, exemplary or consequential damages.
 * 3. No warranty of fitness for particular purpose or non-infringement is provided. */


#include "cli.hpp"

#include "time_utils.hpp"

#include <cstdio>
#include <cstring>
#include <string>

namespace adrm {


    static auto isValidSortField(const std::string & field) -> bool {
        return field == "id" || field == "fname" || field == "fdate" ||
               field == "fsize" || field == "rdate" || field == "cdate" ||
               field == "state";
    }

    auto parseArgs(int argc, char * argv[]) -> ParsedArgs {
        ParsedArgs result;
        bool after_separator = false;
        bool filter_mode = false;

        struct PendingDuration {
            bool active = false;
            bool is_deadline = false;
            std::string value;
            bool has_files_after = false;
        };

        PendingDuration current_dur;

        std::vector<FileWithDuration> collected_files;

        auto flushPendingDuration = [&]() {
            if (current_dur.active && !current_dur.has_files_after) {
                std::fprintf(stderr, "adrm: error: duration '%s%s' has no files after it\n",
                             current_dur.is_deadline ? "--deadline " : "",
                             current_dur.value.c_str());
                result.mode = OperationMode::help;
            }
            current_dur = {};
        };

        for (int i = 1; i < argc; ++i) {
            std::string arg(argv[i]);

            if (after_separator) {
                FileWithDuration fwd;
                fwd.path = arg;
                if (current_dur.active) {
                    fwd.has_duration = true;
                    fwd.is_deadline = current_dur.is_deadline;
                    fwd.duration_str = current_dur.value;
                    fwd.deadline_str = current_dur.is_deadline ? current_dur.value : "";
                    current_dur.has_files_after = true;
                }
                collected_files.push_back(fwd);
                continue;
            }

            if (arg == "--") {
                after_separator = true;
                continue;
            }

            if (arg == "--help" || arg == "-h") {
                result.mode = OperationMode::help;
                return result;
            }

            if (arg == "--version") {
                result.mode = OperationMode::version;
                return result;
            }

            if (arg == "--default") {
                result.mode = OperationMode::generate_default;
                return result;
            }

            if (arg == "--query-all") {
                result.mode = OperationMode::query_all;
                filter_mode = true;
                continue;
            }

            if (arg == "--query") {
                if (i + 1 < argc && std::string(argv[i + 1]) == "--help") {
                    result.mode = OperationMode::query;
                    result.filter_args.emplace_back("--help");
                    return result;
                }
                result.mode = OperationMode::query;
                filter_mode = true;
                continue;
            }

            if (arg == "--restore-all") {
                result.mode = OperationMode::restore_all;
                filter_mode = false;
                continue;
            }

            if (arg == "--restore") {
                if (i + 1 < argc && std::string(argv[i + 1]) == "--help") {
                    result.mode = OperationMode::restore;
                    result.filter_args.emplace_back("--help");
                    return result;
                }
                result.mode = OperationMode::restore;
                filter_mode = true;
                continue;
            }

            if (arg == "--clean-all") {
                result.mode = OperationMode::clean_all;
                filter_mode = false;
                continue;
            }

            if (arg == "--clean") {
                if (i + 1 < argc && std::string(argv[i + 1]) == "--help") {
                    result.mode = OperationMode::clean;
                    result.filter_args.emplace_back("--help");
                    return result;
                }
                result.mode = OperationMode::clean;
                filter_mode = true;
                continue;
            }

            if (arg == "--clear") {
                result.mode = OperationMode::clear;
                return result;
            }

            if (arg == "--reset-db") {
                result.mode = OperationMode::reset_db;
                return result;
            }

            if (arg == "--recursive") {
                result.flag_recursive = true;
                continue;
            }

            if (arg == "--color") {
                result.flag_color = true;
                continue;
            }

            if (arg == "--sort-asc" && i + 1 < argc) {
                ++i;
                if (!isValidSortField(argv[i])) {
                    std::fprintf(stderr, "adrm: invalid sort field '%s'\n", argv[i]);
                    result.mode = OperationMode::help;
                    return result;
                }
                result.sort_specs.push_back({SortDirection::asc, std::string(argv[i])});
                continue;
            }

            if (arg == "--sort-des" && i + 1 < argc) {
                ++i;
                if (!isValidSortField(argv[i])) {
                    std::fprintf(stderr, "adrm: invalid sort field '%s'\n", argv[i]);
                    result.mode = OperationMode::help;
                    return result;
                }
                result.sort_specs.push_back({SortDirection::desc, std::string(argv[i])});
                continue;
            }

            if (filter_mode) {
                if (arg == "--id" || arg == "--items" || arg == "--fname" ||
                    arg == "--fdate" || arg == "--fsize" || arg == "--rdate" ||
                    arg == "--cdate" || arg == "--state") {
                    result.filter_args.push_back(arg);
                    if (i + 1 < argc) {
                        ++i;
                        result.filter_args.push_back(argv[i]);
                    }
                    continue;
                }

                if (arg.size() > 1 && arg[0] == '-' && arg[1] == '-') {
                    result.filter_args.push_back(arg);
                    continue;
                }
            }

            if (arg == "--deadline") {
                if (i + 1 < argc) {
                    ++i;

                    flushPendingDuration();

                    current_dur.active = true;
                    current_dur.is_deadline = true;
                    current_dur.value = argv[i];
                    current_dur.has_files_after = false;
                }
                continue;
            }

            if (arg.size() >= 2 && arg[0] == '-' && arg[1] != '-') {
                for (std::size_t j = 1; j < arg.size(); ++j) {
                    switch (arg[j]) {
                    case 'f':
                        result.flag_force = true;
                        break;
                    case 'i':
                        result.flag_interactive = true;
                        break;
                    case 'I':
                        result.flag_interactive_once = true;
                        break;
                    case 'r':
                    case 'R':
                        result.flag_recursive = true;
                        break;
                    case 'd':
                        result.flag_directory = true;
                        break;
                    case 'v':
                        result.flag_verbose = true;
                        break;
                    default:
                        std::fprintf(stderr, "adrm: invalid option -- '%c'\n", arg[j]);
                        result.mode = OperationMode::help;
                        return result;
                    }
                }
                continue;
            }

            if (!filter_mode && isDurationSpecifier(arg)) {
                flushPendingDuration();

                current_dur.active = true;
                current_dur.is_deadline = false;
                current_dur.value = arg;
                current_dur.has_files_after = false;
                continue;
            }

            if (!filter_mode) {
                FileWithDuration fwd;
                fwd.path = arg;
                if (current_dur.active) {
                    fwd.has_duration = true;
                    fwd.is_deadline = current_dur.is_deadline;
                    fwd.duration_str = current_dur.value;
                    fwd.deadline_str = current_dur.is_deadline ? current_dur.value : "";
                    current_dur.has_files_after = true;
                }
                collected_files.push_back(fwd);
            }
        }

        flushPendingDuration();

        if (result.mode == OperationMode::help)
            return result;

        result.files = std::move(collected_files);

        return result;
    }

    auto printHelp() -> void {
        std::fprintf(stdout,
                     "Usage: adrm [OPTION]... [FILE]...\n"
                     "Move files to trash instead of permanently deleting them.\n"
                     "\n"
                     "By default, adrm does not remove directories. Use -r/-R to remove\n"
                     "directories and their contents recursively.\n"
                     "\n"
                     "Options:\n"
                     "  -f, --force          ignore nonexistent files, bypass ignore rules\n"
                     "  -i                   prompt before every removal\n"
                     "  -I                   prompt once before removing more than 3 files\n"
                     "  -r, -R, --recursive  remove directories and their contents recursively\n"
                     "  -d                   remove empty directories\n"
                     "  -v, --verbose        explain what is being done\n"
                     "      --               end of options, treat all following as file names\n"
                     "\n"
                     "Time specifiers (right-associative, apply to following files):\n"
                     "  +NdNhNmNs           set cleanup time relative to now (e.g., +3d4h5m)\n"
                     "  --deadline DATETIME  set cleanup time to an absolute date/time\n"
                     "\n"
                     "Deadline date formats:\n"
                     "  \"2027-01-01\"          YYYY-MM-DD HH:MM:SS (time optional)\n"
                     "  \"2027/01/01 03:45\"    with / separator\n"
                     "  \"2027_01_01 03:45:01\" with _ separator\n"
                     "\n"
                     "Query/Restore/Clean operations:\n"
                     "  --query              show recent records (default 10)\n"
                     "  --query-all          show all records\n"
                     "  --restore            restore the most recent recycled file\n"
                     "  --restore-all        restore all recycled files\n"
                     "  --clean              permanently delete the most recent recycled file\n"
                     "  --clean-all          permanently delete all recycled files\n"
                     "  --clear              clean all expired files\n"
                     "\n"
                     "Filter conditions (for --query, --restore, --clean):\n"
                     "  --id N               match exact ID\n"
                     "  --id +N              ID >= N\n"
                     "  --id -N              ID <= N\n"
                     "  --items N            limit to N most recent records\n"
                     "  --fname PATTERN      match filename containing PATTERN\n"
                     "  --fdate DURATION     filter by original file modification date\n"
                     "  --fsize SPEC         filter by original file size (e.g., +1g, -10m)\n"
                     "  --rdate DURATION     filter by recycle date\n"
                     "  --cdate DURATION     filter by cleanup date\n"
                     "  --state STATUS       filter by status (recycled/restored/cleaned/exception)\n"
                     "\n"
                     "Sort options (for --query, --query-all):\n"
                     "  --sort-asc FIELD     sort ascending by field (id/fname/fdate/fsize/rdate/cdate/state)\n"
                     "  --sort-des FIELD     sort descending by field\n"
                     "  --color              colorize output (blue=recycled, green=restored, yellow=cleaned, red=exception)\n"
                     "\n"
                     "Other:\n"
                     "  --default            generate default configuration file\n"
                     "  --reset-db           reset the database (only if no recycled files)\n"
                     "  --help               display this help and exit\n"
                     "  --version            output version information and exit\n"
                     "\n"
                     "Environment variables:\n"
                     "  ADRM_HOME         custom adrm home directory (default: ~/.adrm)\n");
    }

    auto printVersion() -> void {
        std::fprintf(stdout, "adrm version %d.%d.%d\n",
                     ADRM_VERSION_MAJOR, ADRM_VERSION_MINOR, ADRM_VERSION_PATCH);
    }

    auto printQueryHelp() -> void {
        std::fprintf(stdout,
                     "Usage: adrm --query [FILTER...] [SORT...]\n"
                     "\n"
                     "Examples:\n"
                     "  adrm --query                   Show recent 10 records\n"
                     "  adrm --query --items 20        Show recent 20 records\n"
                     "  adrm --query --state recycled  Show only recycled files\n"
                     "  adrm --query --fname \"test\"    Show files matching 'test'\n"
                     "  adrm --query --sort-asc id     Sort by id ascending\n"
                     "  adrm --query --color           Colorize output\n"
                     "\n"
                     "Filter conditions can be combined (logical AND):\n"
                     "  adrm --query --state recycled --fsize -1g --id +5\n");
    }

    auto printRestoreHelp() -> void {
        std::fprintf(stdout,
                     "Usage: adrm --restore [FILTER...]\n"
                     "\n"
                     "Examples:\n"
                     "  adrm --restore                 Restore the most recent recycled file\n"
                     "  adrm --restore-all             Restore all recycled files\n"
                     "  adrm --restore --id 5          Restore record with ID 5\n"
                     "  adrm --restore --fname \"test\"  Restore files matching 'test'\n");
    }

    auto printCleanHelp() -> void {
        std::fprintf(stdout,
                     "Usage: adrm --clean [FILTER...]\n"
                     "\n"
                     "Examples:\n"
                     "  adrm --clean                   Clean the most recent recycled file\n"
                     "  adrm --clean-all               Clean all recycled files\n"
                     "  adrm --clean --id 5            Clean record with ID 5\n"
                     "  adrm --clean --state recycled  Clean all recycled files\n");
    }


} // namespace adrm
