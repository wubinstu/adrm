
/**
 * @file:       uuid.hpp
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
#ifndef ADRM_UUID_HPP_
#define ADRM_UUID_HPP_

#include <array>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <random>
#include <string>

namespace adrm {


    inline auto generateUUID() -> std::string {
        std::array<unsigned char, 16> buf {};
        auto * f = std::fopen("/dev/urandom", "rb");
        if (f) {
            std::fread(buf.data(), 1, buf.size(), f);
            std::fclose(f);
        }
        else {
            std::random_device rd;
            std::mt19937 gen {rd()};
            std::uniform_int_distribution<unsigned char> dist {0x00, 0xFF};
            for (unsigned char & i : buf)
                i = static_cast<unsigned char>(dist(gen));
        }

        buf[6] = (buf[6] & 0x0F) | 0x40;
        buf[8] = (buf[8] & 0x3F) | 0x80;

        char out[37];
        std::snprintf(out, sizeof(out),
                      "%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
                      buf[0], buf[1], buf[2], buf[3], buf[4], buf[5], buf[6], buf[7],
                      buf[8], buf[9], buf[10], buf[11], buf[12], buf[13], buf[14], buf[15]);
        return std::string(out);
    }


} // namespace adrm


#endif
