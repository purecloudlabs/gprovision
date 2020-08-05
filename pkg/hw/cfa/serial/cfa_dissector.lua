-- cfa_dissector.lua
-- trivial wireshark packet dissector for crystalfontz
-- See Help | About Wireshark | Folders for the "Personal Lua Plugins" dir you should drop it in

cfa_proto=Proto("cfa","crystalfontz")
function cfa_proto.dissector(buffer,pinfo,tree)
    local incoming=tostring(pinfo.net_dst)=="host"
    local buf_len=buffer:len()
    if incoming and buf_len<=3 then
        pinfo.cols.info="?"
        return
    end
    pinfo.cols.protocol="CFA"
    local subtree=tree:add(cfa_proto,buffer(),"Crystalfontz protocol data")
    local start=0
    if incoming then
        -- there are two extra bytes of data I'm not sure about, so everything is shifted by 2
        start=2
        subtree:add(buffer(0,2),"unknown: "..buffer(0,2))
    end
    local cmd=buffer(start,1):uint()
    local cmd_string=""
    if (cmd < 0x40 ) then
        --command
        cmd_string=cmd_string.."CMD_0x"
    elseif (cmd < 0x80) then
        --normal response
        cmd_string=cmd_string.."OK__0x"
        cmd=cmd-0x40
    elseif (cmd < 0xC0) then
        --report
        cmd_string=cmd_string.."RPT_0x"
        cmd=cmd-0x80
    else
        --error
        cmd_string=cmd_string.."ERR_0x"
        cmd=cmd-0xC0
    end
    cmd_string=cmd_string .. string.format("%x",cmd)
    subtree:add(buffer(start,1),"type: "..cmd_string)
    local data_len=buffer(start+1,1):uint()
    if (data_len+4+start ~= buf_len) then
        subtree:add(buffer(start+1,1),"invalid data_len: " .. string.format("%x",data_len))
    else
        subtree:add(buffer(start+1,1),"data_len: " .. string.format("%x",data_len))
        if (data_len > 0) then
            subtree:add(buffer(start+2,data_len),"data (hex): " .. buffer(start+2,data_len))
            subtree:add(buffer(start+2,data_len),"data (str): " .. buffer(start+2,data_len):string())
        else
            subtree:add(buffer(start+2,0),"data: NA")
        end
        subtree:add(buffer(start+2+data_len,2),"CRC: 0x" .. buffer(start+2+data_len,2):le_uint())
    end
    pinfo.cols.info=cmd_string
end

-- load usb bulk table
usb_bulk=DissectorTable.get("usb.bulk")
-- register our proto; interface class 0xff is likely wrong but not sure what to actually use...
usb_bulk:add(0xffff,cfa_proto)
usb_bulk:add(0xff,cfa_proto)

-- https://osqa-ask.wireshark.org/questions/56135/how-to-call-my-dissector-on-usb-payload-leftover-capture-data
-- https://wiki.wireshark.org/Lua/Dissectors
-- https://desowin.org/usbpcap/dissectors.html
