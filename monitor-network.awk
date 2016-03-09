BEGIN {
	hex["0"] = 0;  hex["1"] = 1;  hex["2"] = 2;  hex["3"] = 3;
	hex["4"] = 4;  hex["5"] = 5;  hex["6"] = 6;  hex["7"] = 7;
	hex["8"] = 8;  hex["9"] = 9;  hex["a"] = 10; hex["b"] = 11;
	hex["c"] = 12; hex["d"] = 13; hex["e"] = 14; hex["f"] = 15;

	types[0] = "BROADCAST"; types[1] = "HELLO"; types[2] = "ADD";
	types[3] = "KICK"; types[4] = "ALIVE";

	next_color = 3;
}

function hex_read_byte(str, pos) {
	return hex[substr(str, pos, 1)] * 16 + hex[substr(str, pos+1, 1)];
}

function hex_read_uint32(str, pos) {
	return hex[substr(str, pos+0, 1)] * 16^7 + hex[substr(str, pos+1, 1)] * 16^6 +\
	       hex[substr(str, pos+2, 1)] * 16^5 + hex[substr(str, pos+3, 1)] * 16^4 +\
	       hex[substr(str, pos+4, 1)] * 16^3 + hex[substr(str, pos+5, 1)] * 16^2 +\
	       hex[substr(str, pos+6, 1)] * 16^1 + hex[substr(str, pos+7, 1)] * 1;
}

function hex_read_ipaddr(str, pos) {
	return hex_read_byte(str, pos) "." hex_read_byte(str, pos+2) "." \
	       hex_read_byte(str, pos+4) "." hex_read_byte(str, pos+6);
}

function color_ip(str) {
	if (colormap[str] == 0 && next_color <= 7) {
		colormap[str] = next_color;
		next_color++;
	}
	return sprintf("\x1b[%dm%s\x1b[0m", 30 + colormap[str], str);
}

function sprintf_data(type, data) {
	gsub(/ /, "", data);
	if (type == 1) {
		right = color_ip(hex_read_ipaddr(data, 1));
		left = color_ip(hex_read_ipaddr(data, 9));
		return sprintf("(new_right %s, new_left %s)", right, left);
	} else if (type == 2) {
		as_right = hex_read_uint32(data, 1);
		as_left = hex_read_uint32(data, 9);
		return sprintf("(as_right %d, as_left %d)", as_right, as_left);
	} else if (type == 3) {
		dead = color_ip(hex_read_ipaddr(data, 1));
		sender = color_ip(hex_read_ipaddr(data, 9));
		return sprintf("(dead %s, sender %s)", dead, sender);
	}
	return "";
}

function sprintf_msg(from, to, id, type, read_count, data) {
	to_from_str = sprintf("%s > %s", color_ip(from), color_ip(to));
	pad_len = 47 - length(to_from_str);
	pad = substr("            ", 1, pad_len);
	decoded_msg = sprintf("(id %10d, type %d, read_count %2d) %s",\
			      id, type, read_count, types[type]);
	if (type == 0 || type == 4) {
		return sprintf("%s%s%s", to_from_str, pad, decoded_msg)
	} else {
		return sprintf("%s%s%s\n    %s %s", to_from_str, pad,\
			       decoded_msg, data, sprintf_data(type, data));
	}
}


/^[0-9]/{
	time = substr($0, 1, 10);
	getline;
	getline;
	getline;
	from =  hex_read_ipaddr($8 $9, 1);
	getline;
	to =  hex_read_ipaddr($2 $3, 1);
	id =  hex_read_uint32($4 $5, 1);
	type = hex_read_uint32($6 $7, 1);
	read_count = hex_read_uint32($8 $9, 1);
	getline;
	data = $4 " " $5 " " $6 " " $7 " " $8 " " $9
	getline;
	data = data " " $2 " " $3;
	
	print time " | " sprintf_msg(from, to, id, type, read_count, data);
	fflush();
}
