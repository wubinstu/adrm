
/**
 * @file:       trash_ops.hpp
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
#ifndef ADRM_TRASH_OPS_HPP_
#define ADRM_TRASH_OPS_HPP_

#include "cli.hpp"
#include "config.hpp"
#include "database.hpp"

namespace adrm {


    class TrashOps {

      public:
        explicit TrashOps(const Config & cfg) noexcept;

        auto init() -> bool;

        auto recycleFiles(const ParsedArgs & args) -> bool;

        auto restoreFiles(const ParsedArgs & args) -> bool;

        auto restoreAll() -> bool;

        auto cleanFiles(const ParsedArgs & args) -> bool;

        auto cleanAll() -> bool;

        auto clearExpired() -> bool;

        auto queryRecords(const ParsedArgs & args) -> bool;

        auto queryAllRecords() -> bool;

        auto resetDatabase() -> bool;

      private:
        Config _m_config;
        Database _m_db;

        auto recycleOne(const FileWithDuration & fwd, bool force, bool recursive, bool verbose) -> bool;

        auto restoreOne(const FileRecord & record, bool verbose) -> bool;

        auto cleanOne(const FileRecord & record, bool verbose) -> bool;

        auto printRecords(const std::vector<FileRecord> & records, bool full) const -> void;

        [[nodiscard]] auto computeDeadline(const FileWithDuration & fwd) const -> std::string;
    };


} // namespace adrm


#endif
