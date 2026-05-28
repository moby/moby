-- https://wiki.wireshark.org/Protobuf
local protobuf_dissector = Dissector.get("protobuf")

local msgtype_field = Field.new("pbf.networkdb.GossipMessage.type")
local msgdata_field = Field.new("pbf.networkdb.GossipMessage.data")

local msgtype_map = {
    [1] = "networkdb.NetworkEvent",
    [2] = "networkdb.TableEvent",
    [3] = "networkdb.NetworkPushPull",
    [4] = "networkdb.BulkSyncMessage",
    [5] = "networkdb.CompoundMessage",
    [6] = "networkdb.NodeEvent",
}

local function last_fieldinfo(field)
    local fieldset = { field() }
    return fieldset[#fieldset]
end

local gossip_proto = Proto("networkdbgossip", "Moby NetworkDB Gossip")
function gossip_proto.dissector(tvb, pinfo, tree)
    pinfo.private["pb_msg_type"] = "message,networkdb.GossipMessage"
    pcall(Dissector.call, protobuf_dissector, tvb, pinfo, tree)

    local msgtype_fieldinfo = last_fieldinfo(msgtype_field)
    if not msgtype_fieldinfo then return end
    local msgdata_fieldinfo = last_fieldinfo(msgdata_field)
    if not msgdata_fieldinfo then return end
    if msgdata_fieldinfo.offset < msgtype_fieldinfo.offset + #msgtype_fieldinfo then return end
    local decodeas = msgtype_map[msgtype_fieldinfo()]
    if not decodeas then return end
    pinfo.private["pb_msg_type"] = "message," .. decodeas
    local ok, err = pcall(Dissector.call, protobuf_dissector, msgdata_fieldinfo.range:tvb(), pinfo, tree)
    if not ok then tree:add_expert_info(PI_DISSECTOR_BUG, PI_ERROR, "Dissector for " .. decodeas .. " failed: " .. tostring(err)) end
end

local tableevent_msgtype_map = {
    ["overlay_peer_table"] = "overlay.PeerRecord",
    ["endpoint_table"] = "libnetwork.EndpointRecord",
}

local tableevent_name_field = Field.new("pbf.networkdb.TableEvent.table_name")
local tableevent_proto = Proto("networkdb-tableevent", "NetworkDB Table Event")
function tableevent_proto.dissector(tvb, pinfo, tree)
    local table_name_fieldinfo = last_fieldinfo(tableevent_name_field)
    if not table_name_fieldinfo then return end
    local decodeas = tableevent_msgtype_map[table_name_fieldinfo()]
    if not decodeas then return end
    pinfo.private["pb_msg_type"] = "message," .. decodeas
    local ok, err = pcall(Dissector.call, protobuf_dissector, tvb, pinfo, tree)
    if not ok then tree:add_expert_info(PI_DISSECTOR_BUG, PI_ERROR, "Dissector for " .. decodeas .. " failed: " .. tostring(err)) end
end


local protobuf_field_table = DissectorTable.get("protobuf_field")
protobuf_field_table:add("networkdb.CompoundMessage.SimpleMessage.Payload", gossip_proto)
protobuf_field_table:add("networkdb.BulkSyncMessage.payload", gossip_proto)
protobuf_field_table:add("networkdb.TableEvent.value", tableevent_proto)
