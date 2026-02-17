// Node.js integration test for gather.optimizer.js
// Usage: node test_gather.js
// Requires: data/data.min.json, tmp/yx518-archive.json

var fs = require('fs');
var path = require('path');

var rawData = JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'data', 'data.min.json'), 'utf8'));
var archive = JSON.parse(fs.readFileSync(path.join(__dirname, 'yx518-archive.json'), 'utf8'));

var skillsMap = {}, equipsMap = {}, ambersMap = {}, disksMap = {};
rawData.skills.forEach(function(s) { skillsMap[s.skillId] = s; });
rawData.equips.forEach(function(e) { equipsMap[e.equipId] = e; });
rawData.ambers.forEach(function(a) { ambersMap[a.amberId] = a; });
rawData.disks.forEach(function(d) { disksMap[d.diskId] = d; });

function getSkillInfo(ids) {
    if (ids == null) return { skillDisp: "", skillEffect: [] };
    if (!Array.isArray(ids)) ids = [ids];
    var desc = "", effects = [];
    ids.forEach(function(sid) {
        var s = skillsMap[sid];
        if (s) { desc += s.desc + "<br>"; effects = effects.concat(s.effect); }
    });
    return { skillDisp: desc, skillEffect: effects };
}

// Build processed equips
var processedEquips = {};
rawData.equips.forEach(function(eq) {
    var si = getSkillInfo(eq.skill);
    processedEquips[eq.equipId] = {
        equipId: eq.equipId, name: eq.name, skill: eq.skill,
        skillDisp: si.skillDisp, effect: si.skillEffect
    };
});

// Build processed ambers with allEffect per level
var processedAmbers = {};
rawData.ambers.forEach(function(amb) {
    var si = getSkillInfo(amb.skill);
    var amp = amb.amplification || 1;
    var allEffect = [];
    for (var lv = 1; lv <= 5; lv++) {
        allEffect.push(si.skillEffect.map(function(eff) {
            return { type: eff.type, value: eff.value + (lv - 1) * amp, condition: eff.condition, cal: eff.cal };
        }));
    }
    processedAmbers[amb.amberId] = {
        amberId: amb.amberId, name: amb.name, type: amb.type,
        amplification: amp, allEffect: allEffect
    };
});

// Build archive lookup
var archChefs = {};
archive.msg.chefs.forEach(function(ac) { archChefs[ac.id] = ac; });

// Build processed chef objects (simulating generateData + importData)
var chefs = rawData.chefs.map(function(raw) {
    var ac = archChefs[raw.chefId];
    var si = getSkillInfo(raw.skill);
    var usi = getSkillInfo(raw.ultimateSkillList || []);

    var chef = {
        chefId: raw.chefId, name: raw.name, rarity: raw.rarity,
        meat: raw.meat || 0, veg: raw.veg || 0, fish: raw.fish || 0, creation: raw.creation || 0,
        specialSkillEffect: si.skillEffect,
        ultimateSkillDisp: usi.skillDisp, ultimateSkillEffect: usi.skillEffect,
        got: ac && ac.got === "是" ? "是" : "",
        ultimate: ac && ac.ult === "是" ? "是" : "",
        equip: ac && ac.equip ? processedEquips[ac.equip] || null : null,
        disk: { level: 0, ambers: [], maxLevel: 1 },
        diskDesc: raw.diskDesc || ""
    };

    var diskDef = disksMap[raw.disk];
    if (diskDef) {
        var level = ac && ac.dlv ? Math.min(ac.dlv, diskDef.maxLevel) : 1;
        var amberIds = ac && ac.ambers ? ac.ambers : [];
        var diskAmbers = [];
        for (var i = 0; i < (diskDef.info ? diskDef.info.length : 0); i++)
            diskAmbers.push({ data: amberIds[i] ? processedAmbers[amberIds[i]] || null : null });
        chef.disk = { level: level, ambers: diskAmbers, maxLevel: diskDef.maxLevel };
    }

    // Compute xVal = base + equip/amber attribute bonuses
    var bonus = { Meat: 0, Vegetable: 0, Creation: 0, Fish: 0 };
    if (chef.equip && chef.equip.effect)
        chef.equip.effect.forEach(function(eff) {
            if (eff.cal === "Abs" && bonus.hasOwnProperty(eff.type)) bonus[eff.type] += eff.value;
        });
    if (chef.disk && chef.disk.ambers && chef.disk.level)
        chef.disk.ambers.forEach(function(a) {
            if (a.data && a.data.allEffect) {
                var lvl = a.data.allEffect[chef.disk.level - 1];
                if (lvl) lvl.forEach(function(eff) {
                    if (eff.cal === "Abs" && bonus.hasOwnProperty(eff.type)) bonus[eff.type] += eff.value;
                });
            }
        });
    chef.meatVal = chef.meat + bonus.Meat;
    chef.vegVal = chef.veg + bonus.Vegetable;
    chef.creationVal = chef.creation + bonus.Creation;
    chef.fishVal = chef.fish + bonus.Fish;
    return chef;
});

// Load gather.optimizer.js
var src = fs.readFileSync(path.join(__dirname, '..', 'js', 'gather.optimizer.js'), 'utf8');
src = src.replace(/\$\(function\(\)\s*\{[\s\S]*?\}\);?\s*$/, '');
eval(src);

// Run optimizer
var gameData = { chefs: chefs, maps: rawData.maps };
var allOwned = [], owned = [];
gameData.chefs.forEach(function(c) {
    if (c.got !== "是") return;
    var gv = gatherComputeChefValue(c);
    var entry = { chef: c, effectiveGain: gv.effectiveGain, flat: gv.flat, critExpected: gv.critExpected, typeBonus: gv.typeBonus };
    allOwned.push(entry);
    var maxPts = Math.max(
        gatherGetPoints(c, "Meat"), gatherGetPoints(c, "Vegetable"),
        gatherGetPoints(c, "Creation"), gatherGetPoints(c, "Fish"));
    if (maxPts <= 12) owned.push(entry);
});
console.log("Owned chefs: " + owned.length + " (all: " + allOwned.length + ")");

var mapDataArr = [];
gameData.maps.forEach(function(m) {
    var cfg = MAP_DEFAULTS[m.name];
    if (!cfg) return;
    mapDataArr.push({
        map: m, name: m.name, category: MAP_CATEGORIES[m.name],
        timeIdx: cfg.timeIdx, chefCount: cfg.chefCount,
        baseRateAll: gatherRateForSkill(m, cfg.timeIdx, 999).rate,
        maxSkill: gatherMaxSkill(m)
    });
});
var DEFAULT_ORDER = ["池塘", "牧场", "猪圈", "森林", "菜地", "菜棚", "鸡舍", "作坊"];
var orderMap = {};
for (var oi = 0; oi < DEFAULT_ORDER.length; oi++) orderMap[DEFAULT_ORDER[oi]] = oi;
mapDataArr.sort(function(a, b) {
    var ia = a.name in orderMap ? orderMap[a.name] : 99;
    var ib = b.name in orderMap ? orderMap[b.name] : 99;
    return ia - ib;
});

var available = owned.slice();
var failures = [], mapResults = [];

mapDataArr.forEach(function(md) {
    var cat = md.category;
    available.sort(function(a, b) { return gatherChefValue(b, cat) - gatherChefValue(a, cat); });
    var team = available.splice(0, md.chefCount);

    // Repair loop
    var repairs = 0;
    while (repairs < md.chefCount) {
        var tg = 0, ts = 0;
        team.forEach(function(t) { tg += gatherChefValue(t, cat); ts += gatherGetPoints(t.chef, cat); });
        if (ts >= md.maxSkill) break;
        var curRate = gatherRateForSkill(md.map, md.timeIdx, ts).rate * (1 + tg / 100);
        var best = null;
        for (var ti = 0; ti < team.length; ti++) {
            var tSk = gatherGetPoints(team[ti].chef, cat), tV = gatherChefValue(team[ti], cat);
            for (var ai = 0; ai < available.length; ai++) {
                var aSk = gatherGetPoints(available[ai].chef, cat);
                if (aSk <= tSk) continue;
                var aV = gatherChefValue(available[ai], cat);
                var nSk = ts - tSk + aSk, nG = tg - tV + aV;
                var nR = gatherRateForSkill(md.map, md.timeIdx, nSk).rate * (1 + nG / 100);
                if (nR > curRate || (!best && nSk > ts)) { curRate = nR; best = { ti: ti, ai: ai }; }
            }
        }
        if (!best) break;
        var tmp = team[best.ti]; team[best.ti] = available[best.ai]; available[best.ai] = tmp;
        repairs++;
    }

    var totalGain = 0, totalSkill = 0;
    team.forEach(function(t) { totalGain += gatherChefValue(t, cat); totalSkill += gatherGetPoints(t.chef, cat); });
    var gr = gatherRateForSkill(md.map, md.timeIdx, totalSkill);
    var enhanced = gr.rate * (1 + totalGain / 100);
    var ok = totalSkill >= md.maxSkill;
    if (!ok) failures.push(md.name);

    var teamStr = team.map(function(t) {
        return t.chef.name + "(" + gatherChefValue(t, cat).toFixed(1) + "% p=" + gatherGetPoints(t.chef, cat) + ")";
    }).join(" + ");
    console.log("\n" + md.name + " (" + cat + ", " + md.chefCount + "人) " + (ok ? "OK" : "FAIL") +
        " 点数:" + totalSkill + "/" + md.maxSkill +
        (repairs > 0 ? " [" + repairs + " swaps]" : ""));
    console.log("  " + teamStr + " = " + totalGain.toFixed(1) + "%");
    if (gr.blocked.length > 0) console.log("  不可采: " + gr.blocked.join(", "));
    console.log("  " + gr.rate.toFixed(2) + " x (1+" + totalGain.toFixed(1) + "%) = " + enhanced.toFixed(2) + "/h");
    mapResults.push({ name: md.name, cat: cat, team: team, totalSkill: totalSkill, maxSkill: md.maxSkill });
});

// Candidates: unassigned chefs with Green slots, top 3 by bonus per category
var assignedIds = {};
mapResults.forEach(function(r) {
    r.team.forEach(function(t) { assignedIds[t.chef.chefId] = r.name; });
});
console.log("\n--- Candidates (Green slots, unassigned) ---");
mapResults.forEach(function(r) {
    var cands = [];
    for (var j = 0; j < owned.length; j++) {
        var e = owned[j];
        if (assignedIds[e.chef.chefId]) continue;
        var gs = gatherGreenSlots(e.chef);
        if (gs.total === 0) continue;
        var pts = gatherGetPoints(e.chef, r.cat);
        if (pts <= 0 && gatherChefValue(e, r.cat) <= 0) continue;
        cands.push({ entry: e, greenTotal: gs.total, greenEmpty: gs.empty });
    }
    cands.sort(function(a, b) {
        var mlA = a.entry.chef.disk && a.entry.chef.disk.maxLevel || 3;
        var mlB = b.entry.chef.disk && b.entry.chef.disk.maxLevel || 3;
        var pa = gatherGetPoints(a.entry.chef, r.cat) + a.greenEmpty * (mlA + 1);
        var pb = gatherGetPoints(b.entry.chef, r.cat) + b.greenEmpty * (mlB + 1);
        if (pa !== pb) return pb - pa;
        return gatherChefValue(b.entry, r.cat) - gatherChefValue(a.entry, r.cat);
    });
    cands = cands.slice(0, 3);
    if (cands.length > 0) {
        console.log(r.name + ":");
        cands.forEach(function(c) {
            var val = gatherChefValue(c.entry, r.cat);
            var pts = gatherGetPoints(c.entry.chef, r.cat);
            console.log("  " + c.entry.chef.name + "(p=" + pts + ", " + val.toFixed(1) + "%, " + c.greenTotal + "绿" + (c.greenEmpty > 0 ? c.greenEmpty + "空" : "") + ")");
        });
    }
});

if (failures.length > 0) {
    console.log("FAIL: skill deficit on " + failures.join(", "));
    process.exit(1);
} else {
    console.log("PASS: all maps meet skill thresholds");
}
