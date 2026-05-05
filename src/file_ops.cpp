/**
 * @file:       file_ops.cpp
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


#include "file_ops.hpp"

#include "time_utils.hpp"

#include <cerrno>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <dirent.h>
#include <ftw.h>
#include <grp.h>
#include <pwd.h>
#include <string>
#include <sys/stat.h>
#include <sys/time.h>
#include <sys/types.h>
#include <unistd.h>

namespace adrm {


    auto getFileMeta(const std::string & path) -> FileMeta {
        FileMeta meta;
        meta.path = path;

        struct stat st {};
        if (stat(path.c_str(), &st) != 0) {
            std::fprintf(stderr, "adrm: cannot stat '%s': %s\n", path.c_str(), std::strerror(errno));
            return meta;
        }

        meta.size_bytes = static_cast<std::uint64_t>(st.st_size);
        meta.size_human = formatSizeHuman(meta.size_bytes);
        meta.perm = static_cast<std::uint16_t>(st.st_mode & 0777);
        meta.special_perm = static_cast<std::uint16_t>(st.st_mode & 07000);
        meta.uid = static_cast<std::uint32_t>(st.st_uid);
        meta.gid = static_cast<std::uint32_t>(st.st_gid);
        meta.owner = getOwnerName(meta.uid);
        meta.group = getGroupName(meta.gid);
        meta.mtime = timeTToDateString(st.st_mtime);
        meta.is_directory = S_ISDIR(st.st_mode) != 0;

        return meta;
    }

    auto getAbsolutePath(const std::string & path) -> std::string {
        auto * res = realpath(path.c_str(), nullptr);
        if (res) {
            std::string result(res);
            std::free(res);
            return result;
        }

        if (path.empty() || path[0] == '/')
            return path;

        char cwd[4096];
        if (getcwd(cwd, sizeof(cwd)))
            return std::string(cwd) + "/" + path;

        return path;
    }

    auto checkReadPermission(const std::string & path) -> bool {
        return access(path.c_str(), R_OK) == 0;
    }

    auto checkWritePermission(const std::string & path) -> bool {
        auto idx = path.find_last_of('/');
        std::string parent;
        if (idx == std::string::npos)
            parent = ".";
        else
            parent = path.substr(0, idx);

        return access(parent.c_str(), W_OK) == 0;
    }

    auto moveFile(const std::string & src, const std::string & dst) -> bool {
        if (std::rename(src.c_str(), dst.c_str()) == 0)
            return true;

        if (errno != EXDEV) {
            std::fprintf(stderr, "adrm: cannot move '%s' to '%s': %s\n",
                         src.c_str(), dst.c_str(), std::strerror(errno));
            return false;
        }

        // Cross-device: copy then delete
        // For simplicity, fall back to system mv command
        std::string cmd = "mv -f -- '";
        for (auto c : src) {
            if (c == '\'')
                cmd += "'\\''";
            else
                cmd += c;
        }
        cmd += "' '";
        for (auto c : dst) {
            if (c == '\'')
                cmd += "'\\''";
            else
                cmd += c;
        }
        cmd += "' 2>/dev/null";

        auto ret = std::system(cmd.c_str());
        if (ret != 0) {
            std::fprintf(stderr, "adrm: failed to move '%s' to '%s'\n", src.c_str(), dst.c_str());
            return false;
        }

        return true;
    }

    namespace {

        int removeCallback(const char * fpath, const struct stat * /*sb*/, int /*typeflag*/, struct FTW * /*ftwbuf*/) {
            return std::remove(fpath);
        }

    } // namespace

    auto deleteFile(const std::string & path) -> bool {
        struct stat st {};
        if (stat(path.c_str(), &st) != 0) {
            std::fprintf(stderr, "adrm: cannot delete '%s': %s\n", path.c_str(), std::strerror(errno));
            return false;
        }

        if (S_ISDIR(st.st_mode)) {
            if (nftw(path.c_str(), removeCallback, 64, FTW_DEPTH | FTW_PHYS) != 0) {
                std::fprintf(stderr, "adrm: failed to delete directory '%s': %s\n",
                             path.c_str(), std::strerror(errno));
                return false;
            }
            return true;
        }

        if (std::remove(path.c_str()) != 0) {
            std::fprintf(stderr, "adrm: failed to delete '%s': %s\n", path.c_str(), std::strerror(errno));
            return false;
        }

        return true;
    }

    auto fileExists(const std::string & path) -> bool {
        struct stat st {};
        return stat(path.c_str(), &st) == 0;
    }

    auto createDirectories(const std::string & path) -> bool {
        if (path.empty())
            return false;

        if (fileExists(path))
            return true;

        auto parent = path;
        auto pos = parent.find_last_of('/');
        if (pos != std::string::npos && pos > 0) {
            auto parentPath = parent.substr(0, pos);
            if (!parentPath.empty() && !fileExists(parentPath)) {
                if (!createDirectories(parentPath))
                    return false;
            }
        }

        if (mkdir(path.c_str(), 0755) != 0 && errno != EEXIST) {
            std::fprintf(stderr, "adrm: cannot create directory '%s': %s\n",
                         path.c_str(), std::strerror(errno));
            return false;
        }

        return true;
    }

    auto setFilePermissions(const std::string & path, std::uint16_t perm, std::uint16_t special_perm) -> bool {
        auto mode = static_cast<mode_t>(perm | special_perm);
        if (chmod(path.c_str(), mode) != 0) {
            std::fprintf(stderr, "adrm: cannot set permissions on '%s': %s\n",
                         path.c_str(), std::strerror(errno));
            return false;
        }
        return true;
    }

    auto setFileOwnership(const std::string & path, std::uint32_t uid, std::uint32_t gid) -> bool {
        if (chown(path.c_str(), static_cast<uid_t>(uid), static_cast<gid_t>(gid)) != 0) {
            std::fprintf(stderr, "adrm: cannot set ownership on '%s': %s\n",
                         path.c_str(), std::strerror(errno));
            return false;
        }
        return true;
    }

    auto setFileModTime(const std::string & path, const std::string & mtime_str) -> bool {
        std::tm tm {};
        std::memset(&tm, 0, sizeof(tm));
        std::sscanf(mtime_str.c_str(), "%d-%d-%d %d:%d:%d",
                    &tm.tm_year, &tm.tm_mon, &tm.tm_mday,
                    &tm.tm_hour, &tm.tm_min, &tm.tm_sec);
        tm.tm_year -= 1900;
        tm.tm_mon -= 1;
        tm.tm_isdst = -1;

        auto t = std::mktime(&tm);
        struct timeval tv[2] {};
        tv[0].tv_sec = t;
        tv[1].tv_sec = t;
        if (utimes(path.c_str(), tv) != 0) {
            std::fprintf(stderr, "adrm: cannot set mtime on '%s': %s\n",
                         path.c_str(), std::strerror(errno));
            return false;
        }
        return true;
    }

    auto isDirectory(const std::string & path) -> bool {
        struct stat st {};
        if (stat(path.c_str(), &st) != 0)
            return false;
        return S_ISDIR(st.st_mode) != 0;
    }

    auto getOwnerName(std::uint32_t uid) -> std::string {
        auto * pw = getpwuid(static_cast<uid_t>(uid));
        if (pw)
            return std::string(pw->pw_name);
        return std::to_string(uid);
    }

    auto getGroupName(std::uint32_t gid) -> std::string {
        auto * gr = getgrgid(static_cast<gid_t>(gid));
        if (gr)
            return std::string(gr->gr_name);
        return std::to_string(gid);
    }


} // namespace adrm
