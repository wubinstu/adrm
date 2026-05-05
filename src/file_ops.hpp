
/**
 * @file:       file_ops.hpp
 * @author:     GLM-5.1-OpenCode
 *
 * @date:       2026-04-30
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
#ifndef ADRM_FILE_OPS_HPP_
#define ADRM_FILE_OPS_HPP_

#include <cstdint>
#include <ctime>
#include <string>
#include <sys/stat.h>

namespace adrm {


    struct FileMeta {
        std::string path;
        std::uint64_t size_bytes = 0;
        std::string size_human;
        std::uint16_t perm = 0;
        std::uint16_t special_perm = 0;
        std::uint32_t uid = 0;
        std::uint32_t gid = 0;
        std::string owner;
        std::string group;
        std::string mtime;
        bool is_directory = false;
    };

    [[nodiscard]] auto getFileMeta(const std::string & path) -> FileMeta;

    [[nodiscard]] auto getAbsolutePath(const std::string & path) -> std::string;

    [[nodiscard]] auto checkReadPermission(const std::string & path) -> bool;

    [[nodiscard]] auto checkWritePermission(const std::string & path) -> bool;

    [[nodiscard]] auto moveFile(const std::string & src, const std::string & dst) -> bool;

    [[nodiscard]] auto deleteFile(const std::string & path) -> bool;

    [[nodiscard]] auto fileExists(const std::string & path) -> bool;

    [[nodiscard]] auto createDirectories(const std::string & path) -> bool;

    auto setFilePermissions(const std::string & path, std::uint16_t perm, std::uint16_t special_perm) -> bool;

    auto setFileOwnership(const std::string & path, std::uint32_t uid, std::uint32_t gid) -> bool;

    auto setFileModTime(const std::string & path, const std::string & mtime_str) -> bool;

    [[nodiscard]] auto isDirectory(const std::string & path) -> bool;

    [[nodiscard]] auto getOwnerName(std::uint32_t uid) -> std::string;

    [[nodiscard]] auto getGroupName(std::uint32_t gid) -> std::string;


} // namespace adrm


#endif
