
/**
 * @file:       cli.hpp
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
#ifndef ADRM_CLI_HPP_
#define ADRM_CLI_HPP_

#include <cstdint>
#include <string>
#include <vector>

namespace adrm {


    enum class OperationMode : std::uint8_t
    {
        remove,
        query_all,
        query,
        restore_all,
        restore,
        clean_all,
        clean,
        clear,
        reset_db,
        generate_default,
        help,
        version,
    };

    struct FileWithDuration {
        std::string path;
        bool has_duration = false;
        bool is_deadline = false;
        std::string duration_str;
        std::string deadline_str;
    };

    struct ParsedArgs {
        OperationMode mode = OperationMode::remove;
        bool flag_force = false;
        bool flag_interactive = false;
        bool flag_interactive_once = false;
        bool flag_recursive = false;
        bool flag_directory = false;
        bool flag_verbose = false;
        std::vector<FileWithDuration> files;
        std::vector<std::string> filter_args;
    };

    [[nodiscard]] auto parseArgs(int argc, char * argv[]) -> ParsedArgs;

    auto printHelp() -> void;

    auto printVersion() -> void;

    auto printQueryHelp() -> void;

    auto printRestoreHelp() -> void;

    auto printCleanHelp() -> void;


} // namespace adrm


#endif
