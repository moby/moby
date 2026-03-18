local msgpack, lzw, crc32 -- Forward declarations for library code inlined at the end of this chunk.

local memberlist_protocol = Proto("Memberlist", "Memberlist Protocol")

local message_type_enum = {
    [0] = "Ping",
    [1] = "IndirectPing",
    [2] = "ACKResp",
    [3] = "Suspect",
    [4] = "Alive",
    [5] = "Dead",
    [6] = "PushPull",
    [7] = "Compound",
    [8] = "User",
    [9] = "Compress",
    [10] = "Encrypt",
    [11] = "NACKResp",
    [12] = "HasCRC",
    [13] = "Err",
    [244] = "HasLabel",
}
local message_type = ProtoField.uint8("memberlist.message_type", "Message Type", base.DEC, message_type_enum)

local crc = ProtoField.uint32("memberlist.crc", "CRC", base.HEX)

local label_size = ProtoField.uint8("memberlist.label_size", "Label Size", base.DEC)
local label = ProtoField.string("memberlist.label", "Label")

local encryption_version = ProtoField.uint8("memberlist.encryption.version", "Encryption Version", base.DEC)
local encrypted_nonce = ProtoField.bytes("memberlist.encryption.nonce", "AES-GCM Nonce", base.NONE)
local encrypted_ciphertext = ProtoField.bytes("memberlist.encryption.ciphertext", "Ciphertext", base.NONE)
local encrypted_tag = ProtoField.bytes("memberlist.encryption.tag", "AEAD tag", base.NONE)
local ciphertext_length = ProtoField.uint32("memberlist.encryption.length", "Ciphertext length", base.DEC)

local decompressed_data = ProtoField.bytes("memberlist.decompressed_data", "Decompressed Data")

local compound_parts = ProtoField.uint8("memberlist.compound.parts", "Compound Parts", base.DEC)
local compound_part_length = ProtoField.uint16("memberlist.compound.part_length", "Compound Part Length", base.DEC)
local userdata = ProtoField.bytes("memberlist.userdata", "User Data", base.NONE)

memberlist_protocol.fields = {
    message_type,
    crc,
    label_size,
    label,
    encryption_version,
    encrypted_nonce,
    encrypted_ciphertext,
    encrypted_tag,
    ciphertext_length,
    decompressed_data,
    compound_parts,
    compound_part_length,
    userdata,
}

local default_settings = { ports = 7946 }

memberlist_protocol.prefs.userdata_dissector = Pref.string("User Data Dissector", "", "Dissector to apply to User message bodies")
memberlist_protocol.prefs.ports = Pref.range("Memberlist TCP+UDP Port(s)", default_settings.ports, "", 65535)
memberlist_protocol.prefs.keylog = Pref.string("Encryption Key Logfile Path", "", [[
Relative or absolute path to a file containing the symmetric keys for decrypting ciphered memberlist messages.
The file should contain the keys, encoded as hex, one per line.]])

local msgpack_string_field = Field.new("msgpack.string")

local function dissect_userdata(buffer, pinfo, tree)
    local dissector = memberlist_protocol.prefs.userdata_dissector
    if dissector ~= "" then
        local d = Dissector.get(dissector)
        if d == nil then
            tree:add(userdata, buffer())
            return
        end
        local success, err = pcall(Dissector.call, d, buffer, pinfo, tree)
        if success then return
        else
            tree:add_expert_info(PI_DISSECTOR_BUG, PI_ERROR, "Dissector " .. dissector .. " failed: " .. tostring(err))
            tree:add(userdata, buffer())
        end
    else
        tree:add(userdata, buffer())
    end
end

local function try_decrypt(buffer, tree, aead)
    local path = memberlist_protocol.prefs.keylog
    if path == "" then return buffer end

    local cryptoversion = buffer(0, 1)
    if cryptoversion:uint() ~= 1 then return buffer end
    local nonce = buffer(1, 12)
    buffer = buffer(13):tvb()
    local tagSize = 16
    local ciphertext = buffer(0, buffer:len()-tagSize)
    local tag = buffer(buffer:len()-tagSize)

    local cipher = GcryptCipher.open(GCRY_CIPHER_AES, GCRY_CIPHER_MODE_GCM, 0)

    -- Don't bother caching the keys in-process.
    -- The filesystem cache is performant enough,
    -- and the OS is better equipped to invalidate the cache.
    for hexkey in io.lines(path) do
        cipher:ctl(GCRYCTL_RESET, ByteArray.new())
        cipher:setkey(ByteArray.new(hexkey))
        cipher:setiv(nonce:bytes())
        if aead then cipher:authenticate(aead) end
        local decrypted = cipher:decrypt(NULL, ciphertext:bytes())
        local result, err = cipher:checktag(tag:bytes())
        if result == 0 then
            local subtree = tree:add(memberlist_protocol, buffer(), "Encrypted Memberlist")
            subtree:add(encryption_version, cryptoversion)
            subtree:add(encrypted_nonce, nonce)
            subtree:add(encrypted_ciphertext, ciphertext)
            subtree:add(encrypted_tag, tag)
            return decrypted:tvb("Decrypted")
        end
    end
    return buffer
end

function memberlist_protocol.dissector(buffer, pinfo, tree)
    local length = buffer:len()
    if length == 0 then return end

    pinfo.cols.protocol = memberlist_protocol.name

    if pinfo.port_type ~= 2 then -- UDP
        buffer = try_decrypt(buffer, tree)
    end

    local opcode = buffer(0, 1):uint()
    local subtree = tree:add(memberlist_protocol, buffer(), "Memberlist Protocol, Type: " .. (message_type_enum[opcode] or tostring(opcode)))

    local msgtype_tree = subtree:add(message_type, buffer(0, 1))
    local buffer = buffer(1):tvb()

    if opcode == 244 and buffer:len() > 0 then -- HasLabel
        local label_length = buffer(0, 1):uint()
        subtree:add(label_size, buffer(0, 1))
        subtree:add(label, buffer(1, label_length))
        memberlist_protocol.dissector(buffer(1 + label_length):tvb(), pinfo, subtree)
        return
    elseif opcode == 12 and buffer:len() > 4 then -- HasCRC
        subtree:add(crc, buffer(0, 4))
        local expected = buffer(0, 4):uint()
        buffer = buffer(4):tvb()
        local actual = crc32(buffer():raw())
        if expected ~= actual then
            subtree:add_expert_info(PI_MALFORMED, PI_ERROR, "Actual CRC32 is " .. tostring(actual))
        end
        memberlist_protocol.dissector(buffer, pinfo, subtree)
        return
    elseif opcode == 6 then -- PushPull
        local msglen = Dissector.get("msgpack"):call(buffer, pinfo, subtree) -- pushPullHeader
        local header = msgpack.decode(buffer(0, msglen):raw())
        buffer = buffer(msglen):tvb()
        for _ = 1, header.Nodes do
            buffer = buffer(Dissector.get("msgpack"):call(buffer, pinfo, subtree)):tvb()
        end
        if header.UserStateLen > 0 then
            dissect_userdata(buffer(0, header.UserStateLen):tvb(), pinfo, tree)
            buffer = buffer(header.UserStateLen):tvb()
        end
    elseif opcode == 7 and buffer:len() >= 1 then -- Compound
        local nparts = buffer(0, 1)
        msgtype_tree:add(compound_parts, nparts)
        nparts = nparts:uint()
        msgtype_tree:append_text(" (" .. nparts .. " parts)")
        buffer = buffer(1):tvb()
        local partlengths = {}
        for i = 0, nparts-1 do
            local partlen = buffer(0, 2)
            msgtype_tree:add(compound_part_length, partlen)
            table.insert(partlengths, partlen:uint())
            buffer = buffer(2):tvb()
        end

        for _, part_length in ipairs(partlengths) do
            if buffer:len() < part_length then
                subtree:add_expert_info(PI_MALFORMED, PI_ERROR, "Truncated part in Compound message")
                return
            end
            local part = buffer(0, part_length):tvb()
            memberlist_protocol.dissector(part, pinfo, subtree)
            buffer = buffer(part_length):tvb()
        end
        return
    elseif opcode == 8 then -- User
        if pinfo.port_type == 2 then -- TCP
            local headerlen = Dissector.get("msgpack"):call(buffer, pinfo, subtree)
            local header = msgpack.decode(buffer(0, headerlen):raw())
            buffer = buffer(headerlen):tvb()
            if header.UserMsgLen ~= nil then
                dissect_userdata(buffer(0, header.UserMsgLen):tvb(), pinfo, tree)
            else
                subtree:add(userdata, buffer())
            end
        else
            dissect_userdata(buffer, pinfo, tree)
        end
        return
    elseif opcode == 10 then -- Encrypt
        local ciphertextlen = buffer(0, 4)
        buffer = buffer(4):tvb()
        msgtype_tree:add(ciphertext_length, ciphertextlen)
        local aead = ByteArray.new("0a") .. ciphertextlen:bytes()
        memberlist_protocol.dissector(try_decrypt(buffer(0, ciphertextlen:uint()), msgtype_tree, aead), pinfo, tree)
        return
    end

    if buffer:len() > 0 then
        Dissector.get("msgpack"):call(buffer, pinfo, subtree)
        if opcode == 9 then -- Compress
            local fields = { msgpack_string_field() }
            for k,v in ipairs(fields) do
                if v.value == "Buf" then
                    local compressed_data = fields[k+3].range
                    local rawdata = compressed_data:raw()
                    local decompressed, err = lzw.decompress(compressed_data:raw())
                    if err ~= nil then
                        subtree:add_expert_info(PI_MALFORMED, PI_ERROR, "Decompression failed: " .. err)
                        return
                    end
                    local dtree = subtree:add(decompressed_data, compressed_data, decompressed)
                    memberlist_protocol.dissector(ByteArray.new(decompressed, true):tvb("Decompressed"), pinfo, tree)
                    return
                end
            end
        end
    end
end

local udp_port = DissectorTable.get("udp.port")
local tcp_port = DissectorTable.get("tcp.port")
function memberlist_protocol.prefs_changed()
    if default_settings.ports ~= memberlist_protocol.prefs.ports then
        udp_port:remove(default_settings.ports, memberlist_protocol)
        tcp_port:remove(default_settings.ports, memberlist_protocol)
        default_settings.ports = memberlist_protocol.prefs.ports
        udp_port:add(default_settings.ports, memberlist_protocol)
        tcp_port:add(default_settings.ports, memberlist_protocol)
    end
end

udp_port:add(default_settings.ports, memberlist_protocol)
tcp_port:add(default_settings.ports, memberlist_protocol)

-------------------------
-- Library definitions --
-------------------------

crc32 = (function()
    local crc32_lut = {
        0x00000000, 0x1db71064, 0x3b6e20c8, 0x26d930ac,
        0x76dc4190, 0x6b6b51f4, 0x4db26158, 0x5005713c,
        0xedb88320, 0xf00f9344, 0xd6d6a3e8, 0xcb61b38c,
        0x9b64c2b0, 0x86d3d2d4, 0xa00ae278, 0xbdbdf21c,
    }

    local function crc32(s, crc)
        if crc == nil then crc = 0 end
        crc = crc ~ 0xffffffff
        for i = 1,#s do
            crc = (crc >> 4) ~ crc32_lut[((crc ~  string.byte(s, i)      ) & 0xf) + 1]
            crc = (crc >> 4) ~ crc32_lut[((crc ~ (string.byte(s, i) >> 4)) & 0xf) + 1]
        end
        return crc ~ 0xffffffff
    end
    return crc32
end)()

lzw = (function()
    --[[
    Port of "compress/lzw".Reader from the Go standard library.

    Copyright 2011 The Go Authors. All rights reserved.
    Use of this source code is governed by a BSD-style
    license that can be found in the LICENSE file.
    ]]--
    local maxWidth = 12
    local decoderInvalidCode = 0xffff
    local flushBuffer = 1 << maxWidth

    local function decompress(compressed)
        local bits, nBits, litWidth = 0, 0, 8
        local width = litWidth + 1
        local clear = 1 << litWidth
        local eof, hi = clear+1, clear+1
        local overflow = 1 << width
        local last = decoderInvalidCode
        local suffix, prefix = {}, {}
        local i = 1
        local output = {}
        local code

        while i <= string.len(compressed) do
            ::continue::
            while nBits < width do
                local x = string.byte(compressed, i)
                i = i+1
                bits = bits | (x << nBits)
                nBits = nBits+8
            end
            code = bits & ((1 << width) - 1)
            bits = bits >> width
            nBits = nBits - width

            if code < clear then -- literal
                table.insert(output, string.char(code))
                if last ~= decoderInvalidCode then
                    suffix[hi] = string.char(code)
                    prefix[hi] = last
                end
            elseif code == clear then -- clear code
                width = litWidth+1
                hi = eof
                overflow = 1 << width
                last = decoderInvalidCode
                goto continue
            elseif code == eof then -- end of file
                return table.concat(output)
            elseif code <= hi then -- code in dictionary
                local c, tmp = code, {}
                if code == hi and last ~= decoderInvalidCode then
                    c = last
                    while c >= clear do
                        c = prefix[c]
                    end
                    table.insert(tmp, string.char(c))
                    c = last
                end
                while c >= clear do
                    table.insert(tmp, suffix[c])
                    c = prefix[c]
                end
                table.insert(tmp, string.char(c))
                for i = #tmp, 1, -1 do
                    table.insert(output, tmp[i])
                end
                if last ~= decoderInvalidCode then
                    suffix[hi] = string.char(c)
                    prefix[hi] = last
                end
            else
                return nil, "Invalid code: " .. code .. " at position " .. i
            end

            last, hi = code, hi + 1
            if hi >= overflow then
                if hi > overflow then
                    return nil, "unreachable"
                end
                if width == maxWidth then
                    last = decoderInvalidCode
                    hi = hi - 1
                else
                    width = width + 1
                    overflow = 1 << width
                end
            end
        end
        return nil, "Unexpected end of input: "..code
    end

    return { decompress = decompress }
end)()

msgpack = (function()
    --[[----------------------------------------------------------------------------

        MessagePack encoder / decoder written in pure Lua 5.3 / Lua 5.4
        written by Sebastian Steinhauer <s.steinhauer@yahoo.de>

        This is free and unencumbered software released into the public domain.

        Anyone is free to copy, modify, publish, use, compile, sell, or
        distribute this software, either in source code form or as a compiled
        binary, for any purpose, commercial or non-commercial, and by any
        means.

        In jurisdictions that recognize copyright laws, the author or authors
        of this software dedicate any and all copyright interest in the
        software to the public domain. We make this dedication for the benefit
        of the public at large and to the detriment of our heirs and
        successors. We intend this dedication to be an overt act of
        relinquishment in perpetuity of all present and future rights to this
        software under copyright law.

        THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
        EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
        MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
        IN NO EVENT SHALL THE AUTHORS BE LIABLE FOR ANY CLAIM, DAMAGES OR
        OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
        ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
        OTHER DEALINGS IN THE SOFTWARE.

        For more information, please refer to <http://unlicense.org/>

    --]]----------------------------------------------------------------------------
    local pack, unpack = string.pack, string.unpack
    local mtype, utf8len = math.type, utf8.len
    local tconcat, tunpack = table.concat, table.unpack
    local ssub = string.sub
    local type, pcall, pairs, select = type, pcall, pairs, select

    --[[----------------------------------------------------------------------------

            Decoder

    --]]----------------------------------------------------------------------------
    local decode_value -- forward declaration

    local function decode_array(data, position, length)
        local elements, value = {}
        for i = 1, length do
            value, position = decode_value(data, position)
            elements[i] = value
        end
        return elements, position
    end

    local function decode_map(data, position, length)
        local elements, key, value = {}
        for i = 1, length do
            key, position = decode_value(data, position)
            value, position = decode_value(data, position)
            elements[key] = value
        end
        return elements, position
    end

    local decoder_functions = {
        [0xc0] = function(data, position)
            return nil, position
        end,
        [0xc2] = function(data, position)
            return false, position
        end,
        [0xc3] = function(data, position)
            return true, position
        end,
        [0xc4] = function(data, position)
            return unpack('>s1', data, position)
        end,
        [0xc5] = function(data, position)
            return unpack('>s2', data, position)
        end,
        [0xc6] = function(data, position)
            return unpack('>s4', data, position)
        end,
        [0xca] = function(data, position)
            return unpack('>f', data, position)
        end,
        [0xcb] = function(data, position)
            return unpack('>d', data, position)
        end,
        [0xcc] = function(data, position)
            return unpack('>B', data, position)
        end,
        [0xcd] = function(data, position)
            return unpack('>I2', data, position)
        end,
        [0xce] = function(data, position)
            return unpack('>I4', data, position)
        end,
        [0xcf] = function(data, position)
            return unpack('>I8', data, position)
        end,
        [0xd0] = function(data, position)
            return unpack('>b', data, position)
        end,
        [0xd1] = function(data, position)
            return unpack('>i2', data, position)
        end,
        [0xd2] = function(data, position)
            return unpack('>i4', data, position)
        end,
        [0xd3] = function(data, position)
            return unpack('>i8', data, position)
        end,
        [0xd9] = function(data, position)
            return unpack('>s1', data, position)
        end,
        [0xda] = function(data, position)
            return unpack('>s2', data, position)
        end,
        [0xdb] = function(data, position)
            return unpack('>s4', data, position)
        end,
        [0xdc] = function(data, position)
            local length
            length, position = unpack('>I2', data, position)
            return decode_array(data, position, length)
        end,
        [0xdd] = function(data, position)
            local length
            length, position = unpack('>I4', data, position)
            return decode_array(data, position, length)
        end,
        [0xde] = function(data, position)
            local length
            length, position = unpack('>I2', data, position)
            return decode_map(data, position, length)
        end,
        [0xdf] = function(data, position)
            local length
            length, position = unpack('>I4', data, position)
            return decode_map(data, position, length)
        end,
    }

    -- add fix-array, fix-map, fix-string, fix-int stuff
    for i = 0x00, 0x7f do
        decoder_functions[i] = function(data, position)
            return i, position
        end
    end
    for i = 0x80, 0x8f do
        decoder_functions[i] = function(data, position)
            return decode_map(data, position, i - 0x80)
        end
    end
    for i = 0x90, 0x9f do
        decoder_functions[i] = function(data, position)
            return decode_array(data, position, i - 0x90)
        end
    end
    for i = 0xa0, 0xbf do
        decoder_functions[i] = function(data, position)
            local length = i - 0xa0
            return ssub(data, position, position + length - 1), position + length
        end
    end
    for i = 0xe0, 0xff do
        decoder_functions[i] = function(data, position)
            return -32 + (i - 0xe0), position
        end
    end

    decode_value = function(data, position)
        local byte, value
        byte, position = unpack('B', data, position)
        value, position = decoder_functions[byte](data, position)
        return value, position
    end


    --[[----------------------------------------------------------------------------

            Interface

    --]]----------------------------------------------------------------------------
    return {
        _AUTHOR = 'Sebastian Steinhauer <s.steinhauer@yahoo.de>',
        _VERSION = '0.6.1',

        -- primary decode function
        decode = function(data, position)
            local values, value, ok = {}
            position = position or 1
            while position <= #data do
                ok, value, position = pcall(decode_value, data, position)
                if ok then
                    values[#values + 1] = value
                else
                    return nil, 'cannot decode MessagePack'
                end
            end
            return tunpack(values)
        end,

        -- decode just one value
        decode_one = function(data, position)
            local value, ok
            ok, value, position = pcall(decode_value, data, position or 1)
            if ok then
                return value, position
            else
                return nil, 'cannot decode MessagePack'
            end
        end,
    }

    --[[----------------------------------------------------------------------------
    --]]----------------------------------------------------------------------------
end)()
