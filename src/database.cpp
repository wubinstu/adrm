/**
 * @file:       database.cpp
 * @author:     WubinWang
 * @contact:    wubinstu@163.com
 * @date:       2026-04-30
 * @license:    MIT License
 *
 * Copyright (c) 2026 WubinWang
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


#include "database.hpp"

#include <cstdio>
#include <cstring>
#include <sqlite3.h>
#include <string>

namespace adrm {


    auto fileStatusToString(FileStatus s) -> const char * {
        switch (s) {
        case FileStatus::recycled:
            return "recycled";
        case FileStatus::restored:
            return "restored";
        case FileStatus::cleaned:
            return "cleaned";
        case FileStatus::exception:
            return "exception";
        }
        return "unknown";
    }

    auto stringToFileStatus(const std::string & s) -> FileStatus {
        if (s == "recycled")
            return FileStatus::recycled;
        if (s == "restored")
            return FileStatus::restored;
        if (s == "cleaned")
            return FileStatus::cleaned;
        if (s == "exception")
            return FileStatus::exception;
        return FileStatus::exception;
    }

    Database::Database(const std::string & db_path) noexcept : _m_db_path {db_path} {
    }

    Database::~Database() noexcept {
        if (this->_m_db) {
            sqlite3_close(static_cast<sqlite3 *>(this->_m_db));
            this->_m_db = nullptr;
        }
    }

    Database::Database(Database && other) noexcept {
        this->_m_db_path = std::move(other._m_db_path);
        this->_m_db = other._m_db;
        other._m_db = nullptr;
    }

    Database & Database::operator=(Database && other) noexcept {
        if (this != &other) {
            if (this->_m_db)
                sqlite3_close(static_cast<sqlite3 *>(this->_m_db));

            this->_m_db_path = std::move(other._m_db_path);
            this->_m_db = other._m_db;
            other._m_db = nullptr;
        }
        return *this;
    }

    auto Database::init() -> bool {
        if (sqlite3_open(this->_m_db_path.c_str(), reinterpret_cast<sqlite3 **>(&this->_m_db)) != SQLITE_OK) {
            std::fprintf(stderr, "adrm: cannot open database '%s': %s\n",
                         this->_m_db_path.c_str(), sqlite3_errmsg(static_cast<sqlite3 *>(this->_m_db)));
            return false;
        }

        auto * db = static_cast<sqlite3 *>(this->_m_db);
        sqlite3_exec(db, "PRAGMA journal_mode=WAL;", nullptr, nullptr, nullptr);
        sqlite3_exec(db, "PRAGMA foreign_keys=ON;", nullptr, nullptr, nullptr);

        const char * create_sql =
            "CREATE TABLE IF NOT EXISTS files ("
            "id INTEGER PRIMARY KEY AUTOINCREMENT,"
            "original_path TEXT NOT NULL,"
            "original_mtime TEXT NOT NULL,"
            "original_size_human TEXT NOT NULL,"
            "original_size_bytes INTEGER NOT NULL,"
            "original_perm INTEGER NOT NULL,"
            "original_special_perm INTEGER NOT NULL,"
            "original_uid INTEGER NOT NULL,"
            "original_gid INTEGER NOT NULL,"
            "original_owner TEXT NOT NULL,"
            "original_group TEXT NOT NULL,"
            "trash_path TEXT NOT NULL,"
            "recycle_time TEXT NOT NULL,"
            "cleanup_time TEXT NOT NULL,"
            "status TEXT NOT NULL DEFAULT 'recycled'"
            ");";

        char * err_msg = nullptr;
        if (sqlite3_exec(db, create_sql, nullptr, nullptr, &err_msg) != SQLITE_OK) {
            std::fprintf(stderr, "adrm: cannot create table: %s\n", err_msg);
            sqlite3_free(err_msg);
            return false;
        }

        return true;
    }

    auto Database::insertRecord(const FileRecord & record) -> bool {
        auto * db = static_cast<sqlite3 *>(this->_m_db);
        if (!db)
            return false;

        const char * sql =
            "INSERT INTO files (original_path, original_mtime, original_size_human, "
            "original_size_bytes, original_perm, original_special_perm, original_uid, "
            "original_gid, original_owner, original_group, trash_path, recycle_time, "
            "cleanup_time, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);";

        sqlite3_stmt * stmt = nullptr;
        if (sqlite3_prepare_v2(db, sql, -1, &stmt, nullptr) != SQLITE_OK) {
            std::fprintf(stderr, "adrm: cannot prepare insert: %s\n", sqlite3_errmsg(db));
            return false;
        }

        sqlite3_bind_text(stmt, 1, record.original_path.c_str(), -1, SQLITE_TRANSIENT);
        sqlite3_bind_text(stmt, 2, record.original_mtime.c_str(), -1, SQLITE_TRANSIENT);
        sqlite3_bind_text(stmt, 3, record.original_size_human.c_str(), -1, SQLITE_TRANSIENT);
        sqlite3_bind_int64(stmt, 4, static_cast<sqlite3_int64>(record.original_size_bytes));
        sqlite3_bind_int(stmt, 5, record.original_perm);
        sqlite3_bind_int(stmt, 6, record.original_special_perm);
        sqlite3_bind_int(stmt, 7, record.original_uid);
        sqlite3_bind_int(stmt, 8, record.original_gid);
        sqlite3_bind_text(stmt, 9, record.original_owner.c_str(), -1, SQLITE_TRANSIENT);
        sqlite3_bind_text(stmt, 10, record.original_group.c_str(), -1, SQLITE_TRANSIENT);
        sqlite3_bind_text(stmt, 11, record.trash_path.c_str(), -1, SQLITE_TRANSIENT);
        sqlite3_bind_text(stmt, 12, record.recycle_time.c_str(), -1, SQLITE_TRANSIENT);
        sqlite3_bind_text(stmt, 13, record.cleanup_time.c_str(), -1, SQLITE_TRANSIENT);
        sqlite3_bind_text(stmt, 14, fileStatusToString(record.status), -1, SQLITE_TRANSIENT);

        auto rc = sqlite3_step(stmt);
        sqlite3_finalize(stmt);

        if (rc != SQLITE_DONE) {
            std::fprintf(stderr, "adrm: insert failed: %s\n", sqlite3_errmsg(db));
            return false;
        }

        return true;
    }

    auto Database::updateStatus(std::int64_t id, FileStatus status) -> bool {
        auto * db = static_cast<sqlite3 *>(this->_m_db);
        if (!db)
            return false;

        const char * sql = "UPDATE files SET status = ? WHERE id = ?;";
        sqlite3_stmt * stmt = nullptr;
        if (sqlite3_prepare_v2(db, sql, -1, &stmt, nullptr) != SQLITE_OK)
            return false;

        sqlite3_bind_text(stmt, 1, fileStatusToString(status), -1, SQLITE_TRANSIENT);
        sqlite3_bind_int64(stmt, 2, id);

        auto rc = sqlite3_step(stmt);
        sqlite3_finalize(stmt);

        return rc == SQLITE_DONE;
    }

    auto Database::updateTrashPath(std::int64_t id, const std::string & new_path) -> bool {
        auto * db = static_cast<sqlite3 *>(this->_m_db);
        if (!db)
            return false;

        const char * sql = "UPDATE files SET trash_path = ? WHERE id = ?;";
        sqlite3_stmt * stmt = nullptr;
        if (sqlite3_prepare_v2(db, sql, -1, &stmt, nullptr) != SQLITE_OK)
            return false;

        sqlite3_bind_text(stmt, 1, new_path.c_str(), -1, SQLITE_TRANSIENT);
        sqlite3_bind_int64(stmt, 2, id);

        auto rc = sqlite3_step(stmt);
        sqlite3_finalize(stmt);

        return rc == SQLITE_DONE;
    }

    auto Database::readRecord(void * stmt_ptr) -> FileRecord {
        FileRecord rec;
        auto * stmt = static_cast<sqlite3_stmt *>(stmt_ptr);

        rec.id = sqlite3_column_int64(stmt, 0);
        rec.original_path = reinterpret_cast<const char *>(sqlite3_column_text(stmt, 1));
        rec.original_mtime = reinterpret_cast<const char *>(sqlite3_column_text(stmt, 2));
        rec.original_size_human = reinterpret_cast<const char *>(sqlite3_column_text(stmt, 3));
        rec.original_size_bytes = static_cast<std::uint64_t>(sqlite3_column_int64(stmt, 4));
        rec.original_perm = static_cast<std::uint16_t>(sqlite3_column_int(stmt, 5));
        rec.original_special_perm = static_cast<std::uint16_t>(sqlite3_column_int(stmt, 6));
        rec.original_uid = static_cast<std::uint32_t>(sqlite3_column_int(stmt, 7));
        rec.original_gid = static_cast<std::uint32_t>(sqlite3_column_int(stmt, 8));
        rec.original_owner = reinterpret_cast<const char *>(sqlite3_column_text(stmt, 9));
        rec.original_group = reinterpret_cast<const char *>(sqlite3_column_text(stmt, 10));
        rec.trash_path = reinterpret_cast<const char *>(sqlite3_column_text(stmt, 11));
        rec.recycle_time = reinterpret_cast<const char *>(sqlite3_column_text(stmt, 12));
        rec.cleanup_time = reinterpret_cast<const char *>(sqlite3_column_text(stmt, 13));

        auto * status_str = reinterpret_cast<const char *>(sqlite3_column_text(stmt, 14));
        if (status_str)
            rec.status = stringToFileStatus(std::string(status_str));

        return rec;
    }

    auto Database::queryAll(std::vector<FileRecord> & out) -> bool {
        auto * db = static_cast<sqlite3 *>(this->_m_db);
        if (!db)
            return false;

        const char * sql = "SELECT * FROM files ORDER BY id DESC;";
        sqlite3_stmt * stmt = nullptr;
        if (sqlite3_prepare_v2(db, sql, -1, &stmt, nullptr) != SQLITE_OK)
            return false;

        while (sqlite3_step(stmt) == SQLITE_ROW)
            out.push_back(this->readRecord(stmt));

        sqlite3_finalize(stmt);
        return true;
    }

    auto Database::queryBySQL(const std::string & where_clause,
                              std::int64_t limit,
                              std::vector<FileRecord> & out) -> bool {
        auto * db = static_cast<sqlite3 *>(this->_m_db);
        if (!db)
            return false;

        std::string sql = "SELECT * FROM files";
        if (!where_clause.empty())
            sql += " WHERE " + where_clause;
        sql += " ORDER BY id DESC";
        if (limit > 0)
            sql += " LIMIT " + std::to_string(limit);

        sqlite3_stmt * stmt = nullptr;
        if (sqlite3_prepare_v2(db, sql.c_str(), -1, &stmt, nullptr) != SQLITE_OK) {
            std::fprintf(stderr, "adrm: query failed: %s\n", sqlite3_errmsg(db));
            return false;
        }

        while (sqlite3_step(stmt) == SQLITE_ROW)
            out.push_back(this->readRecord(stmt));

        sqlite3_finalize(stmt);
        return true;
    }

    auto Database::queryById(std::int64_t id, FileRecord & out) -> bool {
        auto * db = static_cast<sqlite3 *>(this->_m_db);
        if (!db)
            return false;

        const char * sql = "SELECT * FROM files WHERE id = ?;";
        sqlite3_stmt * stmt = nullptr;
        if (sqlite3_prepare_v2(db, sql, -1, &stmt, nullptr) != SQLITE_OK)
            return false;

        sqlite3_bind_int64(stmt, 1, id);

        auto found = false;
        if (sqlite3_step(stmt) == SQLITE_ROW) {
            out = this->readRecord(stmt);
            found = true;
        }

        sqlite3_finalize(stmt);
        return found;
    }

    auto Database::queryLatest(std::int64_t count, std::vector<FileRecord> & out) -> bool {
        auto * db = static_cast<sqlite3 *>(this->_m_db);
        if (!db)
            return false;

        std::string sql = "SELECT * FROM files ORDER BY id DESC LIMIT " + std::to_string(count) + ";";
        sqlite3_stmt * stmt = nullptr;
        if (sqlite3_prepare_v2(db, sql.c_str(), -1, &stmt, nullptr) != SQLITE_OK)
            return false;

        while (sqlite3_step(stmt) == SQLITE_ROW)
            out.push_back(this->readRecord(stmt));

        sqlite3_finalize(stmt);
        return true;
    }

    auto Database::queryLatestRecycled(std::int64_t count, std::vector<FileRecord> & out) -> bool {
        auto * db = static_cast<sqlite3 *>(this->_m_db);
        if (!db)
            return false;

        std::string sql = "SELECT * FROM files WHERE status = 'recycled' ORDER BY id DESC LIMIT " +
                          std::to_string(count) + ";";
        sqlite3_stmt * stmt = nullptr;
        if (sqlite3_prepare_v2(db, sql.c_str(), -1, &stmt, nullptr) != SQLITE_OK)
            return false;

        while (sqlite3_step(stmt) == SQLITE_ROW)
            out.push_back(this->readRecord(stmt));

        sqlite3_finalize(stmt);
        return true;
    }

    auto Database::hasRecycledFiles() -> bool {
        auto * db = static_cast<sqlite3 *>(this->_m_db);
        if (!db)
            return false;

        const char * sql = "SELECT COUNT(*) FROM files WHERE status = 'recycled';";
        sqlite3_stmt * stmt = nullptr;
        if (sqlite3_prepare_v2(db, sql, -1, &stmt, nullptr) != SQLITE_OK)
            return false;

        auto count = 0;
        if (sqlite3_step(stmt) == SQLITE_ROW)
            count = sqlite3_column_int(stmt, 0);

        sqlite3_finalize(stmt);
        return count > 0;
    }

    auto Database::resetDatabase() -> bool {
        auto * db = static_cast<sqlite3 *>(this->_m_db);
        if (!db)
            return false;

        char * err_msg = nullptr;
        if (sqlite3_exec(db, "DELETE FROM files;", nullptr, nullptr, &err_msg) != SQLITE_OK) {
            std::fprintf(stderr, "adrm: failed to reset database: %s\n", err_msg);
            sqlite3_free(err_msg);
            return false;
        }

        sqlite3_exec(db, "DELETE FROM sqlite_sequence WHERE name='files';", nullptr, nullptr, nullptr);
        return true;
    }


} // namespace adrm
