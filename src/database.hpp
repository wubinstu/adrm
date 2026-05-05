
/**
 * @file:       database.hpp
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


#pragma once
#ifndef ADRM_DATABASE_HPP_
#define ADRM_DATABASE_HPP_

#include <cstdint>
#include <string>
#include <vector>

namespace adrm {


    enum class FileStatus : std::uint8_t
    {
        recycled,
        restored,
        cleaned,
        exception,
    };

    [[nodiscard]] auto fileStatusToString(FileStatus s) -> const char *;
    [[nodiscard]] auto stringToFileStatus(const std::string & s) -> FileStatus;

    struct FileRecord {
        std::int64_t id = 0;
        std::string original_path;
        std::string original_mtime;
        std::string original_size_human;
        std::uint64_t original_size_bytes = 0;
        std::uint16_t original_perm = 0;
        std::uint16_t original_special_perm = 0;
        std::uint32_t original_uid = 0;
        std::uint32_t original_gid = 0;
        std::string original_owner;
        std::string original_group;
        std::string trash_path;
        std::string recycle_time;
        std::string cleanup_time;
        FileStatus status = FileStatus::recycled;
    };

    class Database {

      public:
        explicit Database(const std::string & db_path) noexcept;
        ~Database() noexcept;

        Database(const Database & other) noexcept = delete;
        Database & operator=(const Database & other) noexcept = delete;
        Database(Database && other) noexcept;
        Database & operator=(Database && other) noexcept;

        auto init() -> bool;

        auto insertRecord(const FileRecord & record) -> bool;

        auto updateStatus(std::int64_t id, FileStatus status) -> bool;

        auto updateTrashPath(std::int64_t id, const std::string & new_path) -> bool;

        [[nodiscard]] auto queryAll(std::vector<FileRecord> & out) -> bool;

        [[nodiscard]] auto queryBySQL(const std::string & where_clause,
                                      std::int64_t limit,
                                      std::vector<FileRecord> & out) -> bool;

        [[nodiscard]] auto queryById(std::int64_t id, FileRecord & out) -> bool;

        [[nodiscard]] auto queryLatest(std::int64_t count, std::vector<FileRecord> & out) -> bool;

        [[nodiscard]] auto queryLatestRecycled(std::int64_t count, std::vector<FileRecord> & out) -> bool;

        [[nodiscard]] auto hasRecycledFiles() -> bool;

        auto resetDatabase() -> bool;

      private:
        std::string _m_db_path;
        void * _m_db = nullptr;

        auto readRecord(void * stmt) -> FileRecord;
    };


} // namespace adrm


#endif
