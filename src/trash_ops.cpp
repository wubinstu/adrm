/**
 * @file:       trash_ops.cpp
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


#include "trash_ops.hpp"

#include "cli.hpp"
#include "config.hpp"
#include "database.hpp"
#include "file_ops.hpp"
#include "filter.hpp"
#include "ignore.hpp"
#include "time_utils.hpp"
#include "uuid.hpp"

#include <algorithm>
#include <cstdio>
#include <cstring>
#include <ctime>
#include <dirent.h>
#include <iostream>
#include <string>
#include <unistd.h>
#include <vector>

namespace adrm {


    TrashOps::TrashOps(const Config & cfg) noexcept : _m_config {cfg}, _m_db {cfg.database_file} {
    }

    auto TrashOps::init() -> bool {
        if (!createDirectories(this->_m_config.trash_dir))
            return false;

        if (!this->_m_db.init())
            return false;

        return true;
    }

    static auto askYesNo(const char * prompt) -> bool {
        std::fprintf(stdout, "%s", prompt);
        std::fflush(stdout);
        char buf[16];
        if (!std::fgets(buf, sizeof(buf), stdin))
            return false;
        return buf[0] == 'y' || buf[0] == 'Y';
    }

    static auto isDirEmpty(const std::string & path) -> bool {
        auto * dir = opendir(path.c_str());
        if (!dir)
            return true;
        auto empty = true;
        while (auto * entry = readdir(dir)) {
            if (std::strcmp(entry->d_name, ".") != 0 && std::strcmp(entry->d_name, "..") != 0) {
                empty = false;
                break;
            }
        }
        closedir(dir);
        return empty;
    }

    auto TrashOps::recycleFiles(const ParsedArgs & args) -> bool {
        if (args.files.empty()) {
            std::fprintf(stderr, "adrm: missing operand\n");
            return false;
        }

        if (args.flag_interactive_once && args.files.size() > 3) {
            std::fprintf(stdout, "adrm: remove %zu files? ", args.files.size());
            if (!askYesNo(""))
                return true;
        }

        auto all_ok = true;

        for (const auto & fwd : args.files) {
            if (args.flag_interactive) {
                auto abs_path = getAbsolutePath(fwd.path);
                auto is_dir = isDirectory(abs_path);
                std::string type_str = is_dir ? "directory" : "regular file";
                std::string prompt = "adrm: remove " + type_str + " '" + fwd.path + "'? ";
                if (!askYesNo(prompt.c_str()))
                    continue;
            }

            if (!this->recycleOne(fwd, args.flag_force, args.flag_recursive,
                                  args.flag_directory, args.flag_verbose))
                all_ok = false;
        }

        return all_ok;
    }

    auto TrashOps::recycleOne(const FileWithDuration & fwd, bool force, bool recursive,
                              bool allow_empty_dir, bool verbose) -> bool {
        auto abs_path = getAbsolutePath(fwd.path);

        if (!fileExists(abs_path)) {
            if (force) {
                if (verbose)
                    std::fprintf(stdout, "adrm: '%s' does not exist, skipping (force)\n", fwd.path.c_str());
                return true;
            }
            std::fprintf(stderr, "adrm: cannot remove '%s': No such file or directory\n", fwd.path.c_str());
            return false;
        }

        auto is_dir = isDirectory(abs_path);

        if (is_dir && !recursive && !allow_empty_dir) {
            std::fprintf(stderr, "adrm: cannot remove '%s': Is a directory\n", fwd.path.c_str());
            return false;
        }

        if (is_dir && !recursive && allow_empty_dir && !isDirEmpty(abs_path)) {
            std::fprintf(stderr, "adrm: cannot remove '%s': Directory not empty (use -r for recursive)\n",
                         fwd.path.c_str());
            return false;
        }

        if (!force) {
            IgnoreMatcher matcher;
            matcher.loadFromFile(this->_m_config.ignore_file);
            if (matcher.isIgnored(abs_path, is_dir)) {
                std::fprintf(stdout, "adrm: '%s' matches ignore rule, skipping (use -f to override)\n",
                             fwd.path.c_str());
                return true;
            }
        }

        if (!checkReadPermission(abs_path)) {
            std::fprintf(stderr, "adrm: cannot remove '%s': Permission denied\n", fwd.path.c_str());
            return false;
        }

        if (!checkWritePermission(abs_path)) {
            std::fprintf(stderr, "adrm: cannot remove '%s': Permission denied (no write on parent)\n",
                         fwd.path.c_str());
            return false;
        }

        auto meta = getFileMeta(abs_path);

        auto uuid = generateUUID();
        auto display_name = abs_path;
        if (is_dir && display_name.back() != '/')
            display_name += '/';

        auto slash = abs_path.find_last_of('/');
        std::string basename;
        if (slash != std::string::npos)
            basename = abs_path.substr(slash + 1);
        else
            basename = abs_path;

        std::string trash_filename = uuid + "_" + basename;
        if (is_dir)
            trash_filename += "/";

        auto trash_path = this->_m_config.trash_dir + "/" + trash_filename;

        auto trash_path_for_move = trash_path;
        if (is_dir && !trash_path_for_move.empty() && trash_path_for_move.back() == '/')
            trash_path_for_move.pop_back();

        auto parent_dir = trash_path_for_move;
        auto parent_slash = parent_dir.find_last_of('/');
        if (parent_slash != std::string::npos)
            parent_dir = parent_dir.substr(0, parent_slash);

        if (!createDirectories(parent_dir))
            return false;

        if (!moveFile(abs_path, trash_path_for_move)) {
            std::fprintf(stderr, "adrm: failed to move '%s' to trash\n", fwd.path.c_str());

            FileRecord err_record;
            err_record.original_path = display_name;
            err_record.original_mtime = meta.mtime;
            err_record.original_size_human = meta.size_human;
            err_record.original_size_bytes = meta.size_bytes;
            err_record.original_perm = meta.perm;
            err_record.original_special_perm = meta.special_perm;
            err_record.original_uid = meta.uid;
            err_record.original_gid = meta.gid;
            err_record.original_owner = meta.owner;
            err_record.original_group = meta.group;
            err_record.trash_path = trash_path;
            err_record.recycle_time = nowAsString();
            err_record.cleanup_time = this->computeDeadline(fwd);
            err_record.status = FileStatus::exception;
            this->_m_db.insertRecord(err_record);

            return false;
        }

        FileRecord record;
        record.original_path = display_name;
        record.original_mtime = meta.mtime;
        record.original_size_human = meta.size_human;
        record.original_size_bytes = meta.size_bytes;
        record.original_perm = meta.perm;
        record.original_special_perm = meta.special_perm;
        record.original_uid = meta.uid;
        record.original_gid = meta.gid;
        record.original_owner = meta.owner;
        record.original_group = meta.group;
        record.trash_path = trash_path;
        record.recycle_time = nowAsString();
        record.cleanup_time = this->computeDeadline(fwd);
        record.status = FileStatus::recycled;

        if (!this->_m_db.insertRecord(record)) {
            std::fprintf(stderr, "adrm: warning: failed to insert database record for '%s'\n",
                         fwd.path.c_str());
        }

        if (verbose)
            std::fprintf(stdout, "recycled '%s'\n", fwd.path.c_str());

        return true;
    }

    auto TrashOps::computeDeadline(const FileWithDuration & fwd) const -> std::string {
        if (fwd.has_duration) {
            if (fwd.is_deadline) {
                auto tm = parseAbsoluteDate(fwd.deadline_str);
                return tmToString(tm);
            }
            auto dur = parseDuration(fwd.duration_str);
            return durationToDeadline(dur);
        }

        return deadlineFromNow(this->_m_config.default_cleanup_days);
    }

    auto TrashOps::restoreFiles(const ParsedArgs & args) -> bool {
        if (!args.filter_args.empty()) {
            for (const auto & a : args.filter_args) {
                if (a == "--help") {
                    printRestoreHelp();
                    return true;
                }
            }
        }

        bool has_explicit_filter = false;
        for (const auto & a : args.filter_args) {
            if (a == "--id" || a == "--items" || a == "--fname" ||
                a == "--fdate" || a == "--fsize" || a == "--rdate" ||
                a == "--cdate" || a == "--state") {
                has_explicit_filter = true;
                break;
            }
        }

        auto limit = has_explicit_filter ? this->_m_config.query_default_limit : 1;
        auto fc = parseFilterArgs(args.filter_args, limit);
        auto where = buildWhereClause(fc);
        auto actual_limit = buildLimitClause(fc, limit);

        if (!where.empty())
            where = "(" + where + ") AND (status = 'recycled')";
        else
            where = "status = 'recycled'";

        std::vector<FileRecord> records;
        if (!this->_m_db.queryBySQL(where, actual_limit, records))
            return false;

        if (records.empty()) {
            std::fprintf(stdout, "adrm: no matching recycled files to restore\n");
            return true;
        }

        auto all_ok = true;
        for (const auto & rec : records) {
            if (!this->restoreOne(rec, args.flag_verbose))
                all_ok = false;
        }

        return all_ok;
    }

    auto TrashOps::restoreAll() -> bool {
        std::vector<FileRecord> records;
        if (!this->_m_db.queryBySQL("status = 'recycled'", 0, records))
            return false;

        if (records.empty()) {
            std::fprintf(stdout, "adrm: no recycled files to restore\n");
            return true;
        }

        auto all_ok = true;
        for (const auto & rec : records) {
            if (!this->restoreOne(rec, false))
                all_ok = false;
        }

        return all_ok;
    }

    auto TrashOps::restoreOne(const FileRecord & record, bool verbose) -> bool {
        auto trash_path = record.trash_path;
        if (!trash_path.empty() && trash_path.back() == '/')
            trash_path.pop_back();

        if (!fileExists(trash_path)) {
            std::fprintf(stderr, "adrm: trash file not found: '%s'\n", record.trash_path.c_str());
            this->_m_db.updateStatus(record.id, FileStatus::exception);
            return false;
        }

        auto original_path = record.original_path;
        if (!original_path.empty() && original_path.back() == '/')
            original_path.pop_back();

        auto parent_dir = original_path;
        auto slash_pos = parent_dir.find_last_of('/');
        if (slash_pos != std::string::npos)
            parent_dir = parent_dir.substr(0, slash_pos);
        else
            parent_dir = ".";

        if (!parent_dir.empty() && !fileExists(parent_dir)) {
            if (!createDirectories(parent_dir)) {
                std::fprintf(stderr, "adrm: cannot recreate original directory '%s'\n", parent_dir.c_str());
                this->_m_db.updateStatus(record.id, FileStatus::exception);
                return false;
            }
        }

        if (fileExists(original_path)) {
            std::fprintf(stderr, "adrm: original path already exists: '%s'\n", record.original_path.c_str());
            this->_m_db.updateStatus(record.id, FileStatus::exception);
            return false;
        }

        if (!moveFile(trash_path, original_path)) {
            std::fprintf(stderr, "adrm: failed to restore '%s'\n", record.original_path.c_str());
            this->_m_db.updateStatus(record.id, FileStatus::exception);
            return false;
        }

        setFileOwnership(original_path, record.original_uid, record.original_gid);
        setFilePermissions(original_path, record.original_perm, record.original_special_perm);
        setFileModTime(original_path, record.original_mtime);

        this->_m_db.updateStatus(record.id, FileStatus::restored);

        if (verbose)
            std::fprintf(stdout, "restored '%s'\n", record.original_path.c_str());

        return true;
    }

    auto TrashOps::cleanFiles(const ParsedArgs & args) -> bool {
        if (!args.filter_args.empty()) {
            for (const auto & a : args.filter_args) {
                if (a == "--help") {
                    printCleanHelp();
                    return true;
                }
            }
        }

        bool has_explicit_filter = false;
        for (const auto & a : args.filter_args) {
            if (a == "--id" || a == "--items" || a == "--fname" ||
                a == "--fdate" || a == "--fsize" || a == "--rdate" ||
                a == "--cdate" || a == "--state") {
                has_explicit_filter = true;
                break;
            }
        }

        auto limit = has_explicit_filter ? this->_m_config.query_default_limit : 1;
        auto fc = parseFilterArgs(args.filter_args, limit);
        auto where = buildWhereClause(fc);
        auto actual_limit = buildLimitClause(fc, limit);

        if (!where.empty())
            where = "(" + where + ") AND (status = 'recycled')";
        else
            where = "status = 'recycled'";

        std::vector<FileRecord> records;
        if (!this->_m_db.queryBySQL(where, actual_limit, records))
            return false;

        if (records.empty()) {
            std::fprintf(stdout, "adrm: no matching recycled files to clean\n");
            return true;
        }

        auto all_ok = true;
        for (const auto & rec : records) {
            if (!this->cleanOne(rec, args.flag_verbose))
                all_ok = false;
        }

        return all_ok;
    }

    auto TrashOps::cleanAll() -> bool {
        std::vector<FileRecord> records;
        if (!this->_m_db.queryBySQL("status = 'recycled'", 0, records))
            return false;

        if (records.empty()) {
            std::fprintf(stdout, "adrm: no recycled files to clean\n");
            return true;
        }

        auto all_ok = true;
        for (const auto & rec : records) {
            if (!this->cleanOne(rec, false))
                all_ok = false;
        }

        return all_ok;
    }

    auto TrashOps::cleanOne(const FileRecord & record, bool verbose) -> bool {
        auto trash_path = record.trash_path;
        if (!trash_path.empty() && trash_path.back() == '/')
            trash_path.pop_back();

        if (!trash_path.empty() && fileExists(trash_path)) {
            if (!deleteFile(trash_path)) {
                std::fprintf(stderr, "adrm: failed to delete '%s'\n", record.trash_path.c_str());
                this->_m_db.updateStatus(record.id, FileStatus::exception);
                return false;
            }
        }

        this->_m_db.updateStatus(record.id, FileStatus::cleaned);

        if (verbose)
            std::fprintf(stdout, "cleaned '%s'\n", record.original_path.c_str());

        return true;
    }

    auto TrashOps::clearExpired() -> bool {
        std::vector<FileRecord> records;
        if (!this->_m_db.queryBySQL("status = 'recycled'", 0, records))
            return false;

        auto now_str = nowAsString();
        std::vector<FileRecord> expired;

        for (const auto & rec : records) {
            if (rec.cleanup_time <= now_str)
                expired.push_back(rec);
        }

        if (expired.empty()) {
            std::fprintf(stdout, "adrm: no expired files\n");
            return true;
        }

        this->printRecords(expired, false, false);
        if (!askYesNo("adrm: clean all expired files listed above? [y/N] "))
            return true;

        auto cleaned = 0;
        for (const auto & rec : expired) {
            if (this->cleanOne(rec, false))
                ++cleaned;
        }

        std::fprintf(stdout, "adrm: cleaned %d expired file(s)\n", cleaned);
        return true;
    }

    auto TrashOps::queryRecords(const ParsedArgs & args) -> bool {
        if (!args.filter_args.empty()) {
            for (const auto & a : args.filter_args) {
                if (a == "--help") {
                    printQueryHelp();
                    return true;
                }
            }
        }

        auto limit = this->_m_config.query_default_limit;
        auto fc = parseFilterArgs(args.filter_args, limit);
        auto where = buildWhereClause(fc);
        auto actual_limit = buildLimitClause(fc, limit);

        std::string order_by = buildOrderByClause(args.sort_specs);

        std::vector<FileRecord> records;
        if (!this->_m_db.queryBySQL(where, actual_limit, order_by, records))
            return false;

        this->printRecords(records, false, args.flag_color);
        return true;
    }

    auto TrashOps::queryAllRecords(const ParsedArgs & args) -> bool {
        std::string order_by = buildOrderByClause(args.sort_specs);

        std::vector<FileRecord> records;
        if (!this->_m_db.queryBySQL("", 0, order_by, records))
            return false;

        this->printRecords(records, true, args.flag_color);
        return true;
    }

    static auto colorForStatus(FileStatus status) -> const char * {
        switch (status) {
        case FileStatus::recycled:
            return "\033[34m";
        case FileStatus::restored:
            return "\033[32m";
        case FileStatus::cleaned:
            return "\033[33m";
        case FileStatus::exception:
            return "\033[31m";
        }
        return "";
    }

    static const char * COLOR_RESET = "\033[0m";

    auto TrashOps::printRecords(const std::vector<FileRecord> & records, bool full, bool color) -> void {
        if (records.empty()) {
            std::fprintf(stdout, "adrm: no records found\n");
            return;
        }

        std::vector<std::string> columns;
        if (full) {
            columns = {"id", "original_path", "original_mtime", "original_size",
                       "original_perm", "original_special_perm", "original_owner",
                       "original_group", "trash_path", "recycle_time", "cleanup_time", "status"};
        }
        else {
            columns = this->_m_config.query_columns;
        }

        std::vector<std::size_t> widths(columns.size());
        for (std::size_t c = 0; c < columns.size(); ++c)
            widths[c] = columns[c].size();

        std::vector<std::vector<std::string>> rows;
        for (const auto & rec : records) {
            std::vector<std::string> row(columns.size());
            for (std::size_t c = 0; c < columns.size(); ++c) {
                const auto & col = columns[c];
                if (col == "id")
                    row[c] = std::to_string(rec.id);
                else if (col == "original_path")
                    row[c] = rec.original_path;
                else if (col == "original_mtime")
                    row[c] = rec.original_mtime;
                else if (col == "original_size")
                    row[c] = rec.original_size_human;
                else if (col == "original_perm") {
                    char buf[8];
                    std::snprintf(buf, sizeof(buf), "%04o", rec.original_perm);
                    row[c] = std::string(buf);
                }
                else if (col == "original_special_perm") {
                    char buf[8];
                    std::snprintf(buf, sizeof(buf), "%04o", rec.original_special_perm);
                    row[c] = std::string(buf);
                }
                else if (col == "original_owner")
                    row[c] = rec.original_owner;
                else if (col == "original_group")
                    row[c] = rec.original_group;
                else if (col == "trash_path")
                    row[c] = rec.trash_path;
                else if (col == "recycle_time")
                    row[c] = rec.recycle_time;
                else if (col == "cleanup_time")
                    row[c] = rec.cleanup_time;
                else if (col == "status")
                    row[c] = fileStatusToString(rec.status);

                if (row[c].size() > widths[c])
                    widths[c] = row[c].size();
            }
            rows.push_back(row);
        }

        for (std::size_t c = 0; c < columns.size(); ++c) {
            if (c > 0)
                std::fprintf(stdout, "  ");
            std::fprintf(stdout, "%-*s", static_cast<int>(widths[c]), columns[c].c_str());
        }
        std::fprintf(stdout, "\n");

        for (std::size_t c = 0; c < columns.size(); ++c) {
            if (c > 0)
                std::fprintf(stdout, "  ");
            for (std::size_t j = 0; j < widths[c]; ++j)
                std::fprintf(stdout, "-");
        }
        std::fprintf(stdout, "\n");

        for (std::size_t r = 0; r < rows.size(); ++r) {
            const auto & row = rows[r];
            auto row_color = color ? colorForStatus(records[r].status) : "";
            auto reset = (color && row_color[0] != '\0') ? COLOR_RESET : "";

            for (std::size_t c = 0; c < row.size(); ++c) {
                if (c > 0)
                    std::fprintf(stdout, "  ");
                if (color && columns[c] == "status" && row_color[0] != '\0')
                    std::fprintf(stdout, "%s%-*s%s", row_color, static_cast<int>(widths[c]), row[c].c_str(), reset);
                else
                    std::fprintf(stdout, "%-*s", static_cast<int>(widths[c]), row[c].c_str());
            }
            std::fprintf(stdout, "\n");
        }
    }

    auto TrashOps::resetDatabase() -> bool {
        if (this->_m_db.hasRecycledFiles()) {
            std::fprintf(stdout, "adrm: there are still files with 'recycled' status:\n");

            std::vector<FileRecord> records;
            (void)this->_m_db.queryBySQL("status = 'recycled'", 0, records);
            this->printRecords(records, false, false);

            if (!askYesNo("adrm: clean all recycled files and reset database? [y/N] "))
                return false;

            for (const auto & rec : records)
                this->cleanOne(rec, false);
        }

        if (!this->_m_db.resetDatabase()) {
            std::fprintf(stderr, "adrm: failed to reset database\n");
            return false;
        }

        std::fprintf(stdout, "adrm: database has been reset\n");
        return true;
    }


} // namespace adrm
