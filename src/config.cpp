/**
 * @file:       config.cpp
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


#include "config.hpp"

#include "file_ops.hpp"

#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <fstream>
#include <sstream>
#include <string>
#include <vector>

namespace adrm {


    auto getAdrmHome() -> std::string {
        auto * env = std::getenv("SAFE_RM_HOME");
        if (env && env[0] != '\0')
            return std::string(env);

        auto * home = std::getenv("HOME");
        if (home && home[0] != '\0')
            return std::string(home) + "/.adrm";

        return "/root/.adrm";
    }

    auto resolvePath(const std::string & base, const std::string & relative_or_absolute) -> std::string {
        if (relative_or_absolute.empty())
            return base;

        if (relative_or_absolute[0] == '/')
            return relative_or_absolute;

        if (relative_or_absolute.size() >= 2 && relative_or_absolute[0] == '~' && relative_or_absolute[1] == '/') {
            auto * home = std::getenv("HOME");
            if (home)
                return std::string(home) + relative_or_absolute.substr(1);
        }

        return base + "/" + relative_or_absolute;
    }

    static auto trimString(const std::string & s) -> std::string {
        std::size_t start = 0;
        while (start < s.size() && (s[start] == ' ' || s[start] == '\t'))
            ++start;
        std::size_t end = s.size();
        while (end > start && (s[end - 1] == ' ' || s[end - 1] == '\t' || s[end - 1] == '\r' || s[end - 1] == '\n'))
            --end;
        return s.substr(start, end - start);
    }

    static auto splitCSV(const std::string & s) -> std::vector<std::string> {
        std::vector<std::string> result;
        std::istringstream iss(s);
        std::string token;
        while (std::getline(iss, token, ',')) {
            auto trimmed = trimString(token);
            if (!trimmed.empty())
                result.push_back(trimmed);
        }
        return result;
    }

    auto loadConfig() -> Config {
        Config cfg;
        cfg.home_dir = getAdrmHome();
        cfg.config_file = cfg.home_dir + "/config";
        cfg.ignore_file = "ignore";
        cfg.database_file = "files.db";
        cfg.trash_dir = "trash";
        cfg.default_cleanup_days = 30;
        cfg.query_default_limit = 10;
        cfg.query_columns = {"id", "original_path", "original_size", "recycle_time", "cleanup_time", "status"};

        std::ifstream ifs(cfg.config_file);
        if (!ifs.is_open())
            return cfg;

        std::string line;
        while (std::getline(ifs, line)) {
            auto trimmed = trimString(line);
            if (trimmed.empty() || trimmed[0] == '#')
                continue;

            auto eq = trimmed.find('=');
            if (eq == std::string::npos)
                continue;

            auto key = trimString(trimmed.substr(0, eq));
            auto val = trimString(trimmed.substr(eq + 1));

            if (key == "default_cleanup_days") {
                cfg.default_cleanup_days = std::atoll(val.c_str());
            }
            else if (key == "trash_dir") {
                cfg.trash_dir = val;
            }
            else if (key == "database_file") {
                cfg.database_file = val;
            }
            else if (key == "ignore_file") {
                cfg.ignore_file = val;
            }
            else if (key == "query_columns") {
                cfg.query_columns = splitCSV(val);
            }
            else if (key == "query_default_limit") {
                cfg.query_default_limit = std::atoll(val.c_str());
            }
        }

        cfg.trash_dir = resolvePath(cfg.home_dir, cfg.trash_dir);
        cfg.database_file = resolvePath(cfg.home_dir, cfg.database_file);
        cfg.ignore_file = resolvePath(cfg.home_dir, cfg.ignore_file);

        return cfg;
    }

    auto generateDefaultConfig() -> bool {
        auto home = getAdrmHome();
        if (!createDirectories(home))
            return false;

        auto configPath = home + "/config";

        if (fileExists(configPath)) {
            std::fprintf(stdout, "adrm: config file already exists at %s\n", configPath.c_str());
            return true;
        }

        std::ofstream ofs(configPath);
        if (!ofs.is_open()) {
            std::fprintf(stderr, "adrm: cannot create config file %s\n", configPath.c_str());
            return false;
        }

        ofs << "# adrm configuration file\n";
        ofs << "# Lines starting with # are comments\n";
        ofs << "# Empty lines are ignored\n";
        ofs << "\n";
        ofs << "# Default cleanup period in days\n";
        ofs << "default_cleanup_days = 30\n";
        ofs << "\n";
        ofs << "# Trash directory (absolute path or relative to adrm home directory)\n";
        ofs << "trash_dir = trash\n";
        ofs << "\n";
        ofs << "# Database file (absolute path or relative to adrm home directory)\n";
        ofs << "database_file = files.db\n";
        ofs << "\n";
        ofs << "# Ignore rules file (absolute path or relative to adrm home directory)\n";
        ofs << "ignore_file = ignore\n";
        ofs << "\n";
        ofs << "# Columns to display in --query output (comma-separated)\n";
        ofs << "# Available: id, original_path, original_mtime, original_size,\n";
        ofs << "#            original_perm, original_special_perm, original_owner,\n";
        ofs << "#            original_group, trash_path, recycle_time, cleanup_time, status\n";
        ofs << "query_columns = id, original_path, original_size, recycle_time, cleanup_time, status\n";
        ofs << "\n";
        ofs << "# Number of records shown by default with --query\n";
        ofs << "query_default_limit = 10\n";

        ofs.close();

        std::fprintf(stdout, "adrm: default config generated at %s\n", configPath.c_str());
        return true;
    }

    auto ensureAdrmDirectories() -> bool {
        auto home = getAdrmHome();
        if (!createDirectories(home))
            return false;
        return true;
    }


} // namespace adrm
