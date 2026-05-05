
/**
 * @file:       main.cpp
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


#include "cli.hpp"
#include "config.hpp"
#include "trash_ops.hpp"

#include <cstdio>

auto main(int argc, char * argv[]) -> int {
    auto args = adrm::parseArgs(argc, argv);

    if (args.mode == adrm::OperationMode::help) {
        adrm::printHelp();
        return 0;
    }

    if (args.mode == adrm::OperationMode::version) {
        adrm::printVersion();
        return 0;
    }

    if (args.mode == adrm::OperationMode::generate_default) {
        if (!adrm::generateDefaultConfig())
            return 1;
        return 0;
    }

    if (!adrm::ensureAdrmDirectories())
        return 1;

    auto cfg = adrm::loadConfig();

    adrm::TrashOps ops(cfg);
    if (!ops.init())
        return 1;

    switch (args.mode) {
    case adrm::OperationMode::remove:
        return ops.recycleFiles(args) ? 0 : 1;

    case adrm::OperationMode::query_all:
        return ops.queryAllRecords() ? 0 : 1;

    case adrm::OperationMode::query:
        return ops.queryRecords(args) ? 0 : 1;

    case adrm::OperationMode::restore_all:
        return ops.restoreAll() ? 0 : 1;

    case adrm::OperationMode::restore:
        return ops.restoreFiles(args) ? 0 : 1;

    case adrm::OperationMode::clean_all:
        return ops.cleanAll() ? 0 : 1;

    case adrm::OperationMode::clean:
        return ops.cleanFiles(args) ? 0 : 1;

    case adrm::OperationMode::clear:
        return ops.clearExpired() ? 0 : 1;

    case adrm::OperationMode::reset_db:
        return ops.resetDatabase() ? 0 : 1;

    default:
        adrm::printHelp();
        return 0;
    }
}
