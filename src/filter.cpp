/**
 * @file:       filter.cpp
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


#include "filter.hpp"

#include "time_utils.hpp"

#include <cstdio>
#include <cstring>
#include <ctime>
#include <string>

namespace adrm {


    static auto buildDateSQL(const std::string & column, const std::string & value) -> std::string {
        auto df = parseDateFilter(value);
        std::string sql;

        if (df.is_relative) {
            auto now = std::time(nullptr);
            auto target = now - df.relative.toSeconds();
            auto target_tm = std::localtime(&target);
            char target_str[32];
            std::strftime(target_str, sizeof(target_str), "%Y-%m-%d %H:%M:%S", target_tm);

            switch (df.op) {
            case FilterOp::exact: {
                auto before = target - 60;
                auto after = target + 60;
                auto before_tm = std::localtime(&before);
                auto after_tm = std::localtime(&after);
                char before_str[32], after_str[32];
                std::strftime(before_str, sizeof(before_str), "%Y-%m-%d %H:%M:%S", before_tm);
                std::strftime(after_str, sizeof(after_str), "%Y-%m-%d %H:%M:%S", after_tm);
                sql = column + " >= '" + std::string(before_str) + "' AND " +
                      column + " <= '" + std::string(after_str) + "'";
                break;
            }
            case FilterOp::gte:
                sql = column + " <= '" + std::string(target_str) + "'";
                break;
            case FilterOp::lte:
                sql = column + " >= '" + std::string(target_str) + "'";
                break;
            }
        }
        else {
            auto target_str = tmToString(df.absolute);

            switch (df.op) {
            case FilterOp::exact:
                sql = column + " = '" + target_str + "'";
                break;
            case FilterOp::gte:
                sql = column + " >= '" + target_str + "'";
                break;
            case FilterOp::lte:
                sql = column + " <= '" + target_str + "'";
                break;
            }
        }

        return sql;
    }

    auto parseFilterArgs(const std::vector<std::string> & args, std::int64_t default_limit) -> FilterConditions {
        FilterConditions fc;

        for (std::size_t i = 0; i < args.size(); ++i) {
            const auto & arg = args[i];

            if (arg == "--id" && i + 1 < args.size()) {
                ++i;
                const auto & val = args[i];
                if (val[0] == '+') {
                    fc.has_id_min = true;
                    fc.id_min = std::atoll(val.c_str() + 1);
                }
                else if (val[0] == '-') {
                    fc.has_id_max = true;
                    fc.id_max = std::atoll(val.c_str() + 1);
                }
                else {
                    fc.has_id_exact = true;
                    fc.id_exact = std::atoll(val.c_str());
                }
            }
            else if (arg == "--items" && i + 1 < args.size()) {
                ++i;
                fc.has_items = true;
                fc.items = std::atoll(args[i].c_str());
            }
            else if (arg == "--fname" && i + 1 < args.size()) {
                ++i;
                fc.has_fname = true;
                fc.fname = args[i];
            }
            else if (arg == "--fdate" && i + 1 < args.size()) {
                ++i;
                fc.has_fdate = true;
                fc.fdate_sql = buildDateSQL("original_mtime", args[i]);
            }
            else if (arg == "--fsize" && i + 1 < args.size()) {
                ++i;
                auto sf = parseSizeSpec(args[i]);
                fc.has_fsize = true;
                switch (sf.op) {
                case FilterOp::exact:
                    fc.fsize_sql = "original_size_bytes = " + std::to_string(sf.bytes);
                    break;
                case FilterOp::gte:
                    fc.fsize_sql = "original_size_bytes >= " + std::to_string(sf.bytes);
                    break;
                case FilterOp::lte:
                    fc.fsize_sql = "original_size_bytes <= " + std::to_string(sf.bytes);
                    break;
                }
            }
            else if (arg == "--rdate" && i + 1 < args.size()) {
                ++i;
                fc.has_rdate = true;
                fc.rdate_sql = buildDateSQL("recycle_time", args[i]);
            }
            else if (arg == "--cdate" && i + 1 < args.size()) {
                ++i;
                fc.has_cdate = true;
                fc.cdate_sql = buildDateSQL("cleanup_time", args[i]);
            }
            else if (arg == "--state" && i + 1 < args.size()) {
                ++i;
                fc.has_state = true;
                fc.state = args[i];
            }
        }

        (void)default_limit;
        return fc;
    }

    auto buildWhereClause(const FilterConditions & fc) -> std::string {
        std::vector<std::string> conditions;

        if (fc.has_id_exact)
            conditions.push_back("id = " + std::to_string(fc.id_exact));
        if (fc.has_id_min)
            conditions.push_back("id >= " + std::to_string(fc.id_min));
        if (fc.has_id_max)
            conditions.push_back("id <= " + std::to_string(fc.id_max));
        if (fc.has_fname)
            conditions.push_back("original_path LIKE '%" + fc.fname + "%'");
        if (fc.has_fdate)
            conditions.push_back(fc.fdate_sql);
        if (fc.has_fsize)
            conditions.push_back(fc.fsize_sql);
        if (fc.has_rdate)
            conditions.push_back(fc.rdate_sql);
        if (fc.has_cdate)
            conditions.push_back(fc.cdate_sql);
        if (fc.has_state)
            conditions.push_back("status = '" + fc.state + "'");

        if (conditions.empty())
            return "";

        std::string where;
        for (std::size_t i = 0; i < conditions.size(); ++i) {
            if (i > 0)
                where += " AND ";
            where += "(" + conditions[i] + ")";
        }

        return where;
    }

    auto buildLimitClause(const FilterConditions & fc, std::int64_t default_limit) -> std::int64_t {
        if (fc.has_items && fc.items > 0)
            return fc.items;
        return default_limit;
    }


} // namespace adrm
