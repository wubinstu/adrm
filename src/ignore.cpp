/**
 * @file:       ignore.cpp
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


#include "ignore.hpp"

#include "file_ops.hpp"

#include <cstdio>
#include <fstream>
#include <string>

namespace adrm {


    auto IgnoreMatcher::loadFromFile(const std::string & path) -> bool {
        if (!fileExists(path))
            return true;

        std::ifstream ifs(path);
        if (!ifs.is_open()) {
            std::fprintf(stderr, "adrm: cannot open ignore file '%s'\n", path.c_str());
            return false;
        }

        std::string line;
        while (std::getline(ifs, line)) {
            if (line.empty() || line[0] == '#')
                continue;

            IgnoreRule rule;
            std::size_t pos = 0;

            if (pos < line.size() && line[pos] == '!') {
                rule.negated = true;
                ++pos;
            }

            auto pattern = line.substr(pos);

            if (!pattern.empty() && pattern.back() == '/') {
                rule.dir_only = true;
                pattern.pop_back();
            }

            while (!pattern.empty() && pattern.back() == ' ')
                pattern.pop_back();

            rule.pattern = pattern;
            if (!rule.pattern.empty())
                this->_m_rules.push_back(rule);
        }

        return true;
    }

    auto IgnoreMatcher::isIgnored(const std::string & path, bool is_dir) const -> bool {
        auto name = path;
        auto slash = name.find_last_of('/');
        if (slash != std::string::npos)
            name = name.substr(slash + 1);

        if (name.empty())
            return false;

        auto result = false;

        for (const auto & rule : this->_m_rules) {
            if (rule.dir_only && !is_dir)
                continue;

            if (matchPattern(rule.pattern, name)) {
                result = !rule.negated;
            }
        }

        return result;
    }

    auto IgnoreMatcher::matchPattern(const std::string & pattern, const std::string & name) -> bool {
        if (pattern == name)
            return true;

        if (pattern == "*")
            return true;

        if (pattern.size() >= 2 && pattern.front() == '*' && pattern.back() == '*') {
            auto middle = pattern.substr(1, pattern.size() - 2);
            if (name.find(middle) != std::string::npos)
                return true;
        }

        if (pattern.size() >= 2 && pattern.front() == '*' && pattern.back() != '*') {
            auto suffix = pattern.substr(1);
            if (name.size() >= suffix.size()) {
                auto ending = name.substr(name.size() - suffix.size());
                if (ending == suffix)
                    return true;
            }
        }

        if (pattern.size() >= 2 && pattern.front() != '*' && pattern.back() == '*') {
            auto prefix = pattern.substr(0, pattern.size() - 1);
            if (name.size() >= prefix.size()) {
                auto beginning = name.substr(0, prefix.size());
                if (beginning == prefix)
                    return true;
            }
        }

        if (pattern.find('?') != std::string::npos) {
            if (pattern.size() == name.size()) {
                auto match = true;
                for (std::size_t i = 0; i < pattern.size(); ++i) {
                    if (pattern[i] != '?' && pattern[i] != name[i]) {
                        match = false;
                        break;
                    }
                }
                if (match)
                    return true;
            }
        }

        if (pattern.find('*') == std::string::npos && pattern.find('?') == std::string::npos) {
            return pattern == name;
        }

        return false;
    }


} // namespace adrm
