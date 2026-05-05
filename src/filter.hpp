
/**
 * @file:       filter.hpp
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


#pragma once
#ifndef ADRM_FILTER_HPP_
#define ADRM_FILTER_HPP_

#include "cli.hpp"

#include <cstdint>
#include <string>
#include <vector>

namespace adrm {


    struct FilterConditions {
        bool has_id_exact = false;
        std::int64_t id_exact = 0;
        bool has_id_min = false;
        std::int64_t id_min = 0;
        bool has_id_max = false;
        std::int64_t id_max = 0;
        bool has_items = false;
        std::int64_t items = 0;
        bool has_fname = false;
        std::string fname;
        bool has_fdate = false;
        std::string fdate_sql;
        bool has_fsize = false;
        std::string fsize_sql;
        bool has_rdate = false;
        std::string rdate_sql;
        bool has_cdate = false;
        std::string cdate_sql;
        bool has_state = false;
        std::string state;
    };

    [[nodiscard]] auto parseFilterArgs(const std::vector<std::string> & args,
                                       std::int64_t default_limit) -> FilterConditions;

    [[nodiscard]] auto buildWhereClause(const FilterConditions & fc) -> std::string;

    [[nodiscard]] auto buildLimitClause(const FilterConditions & fc, std::int64_t default_limit) -> std::int64_t;

    [[nodiscard]] auto buildOrderByClause(const std::vector<SortSpec> & sort_specs) -> std::string;

    [[nodiscard]] auto sortFieldToColumn(const std::string & field) -> std::string;


} // namespace adrm


#endif
