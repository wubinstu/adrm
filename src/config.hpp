
/**
 * @file:       config.hpp
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
#ifndef ADRM_CONFIG_HPP_
#define ADRM_CONFIG_HPP_

#include <cstdint>
#include <string>
#include <vector>

namespace adrm {


    struct Config {
        std::string home_dir;
        std::string config_file;
        std::string ignore_file;
        std::string database_file;
        std::string trash_dir;
        std::int64_t default_cleanup_days = 30;
        std::vector<std::string> query_columns;
        std::int64_t query_default_limit = 10;
    };

    [[nodiscard]] auto getAdrmHome() -> std::string;

    [[nodiscard]] auto loadConfig() -> Config;

    auto resolvePath(const std::string & base, const std::string & relative_or_absolute) -> std::string;

    auto generateDefaultConfig() -> bool;

    auto ensureAdrmDirectories() -> bool;


} // namespace adrm


#endif
