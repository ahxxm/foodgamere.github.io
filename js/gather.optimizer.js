// Gathering optimizer — assigns owned chefs to 8 maps to maximize total material output rate.
// Uses calCustomRule.gameData (set by food.min.js after init).

var MAP_CATEGORIES = {
    "牧场": "Meat", "鸡舍": "Meat", "猪圈": "Meat",
    "菜棚": "Vegetable", "菜地": "Vegetable", "森林": "Vegetable",
    "作坊": "Creation",
    "池塘": "Fish"
};

var MAP_DEFAULTS = {
    "牧场": { timeIdx: 3, chefCount: 4 },
    "鸡舍": { timeIdx: 3, chefCount: 4 },
    "猪圈": { timeIdx: 3, chefCount: 4 },
    "菜棚": { timeIdx: 3, chefCount: 4 },
    "菜地": { timeIdx: 3, chefCount: 4 },
    "森林": { timeIdx: 3, chefCount: 4 },
    "作坊": { timeIdx: 3, chefCount: 4 },
    "池塘": { timeIdx: 4, chefCount: 5 }
};

var GATHER_CAT_TYPES = {
    "Material_Meat": "Meat",
    "Material_Vegetable": "Vegetable",
    "Material_Creation": "Creation",
    "Material_Fish": "Fish"
};

var EQUIP_CRIT_RE = /(\d+)%概率.*?(\d+)%素材/;

// User-defined map priority order (null = use default)
var _gatherMapOrder = null;

function gatherComputeChefValue(chef) {
    var flat = 0, critExpected = 0;
    var typeBonus = { Meat: 0, Vegetable: 0, Creation: 0, Fish: 0 };

    if (chef.specialSkillEffect) {
        for (var i = 0; i < chef.specialSkillEffect.length; i++) {
            var eff = chef.specialSkillEffect[i];
            if (eff.type === "Material_Gain" && eff.condition === "Self")
                flat += eff.value;
            var cat = GATHER_CAT_TYPES[eff.type];
            if (cat && eff.condition === "Self") typeBonus[cat] += eff.value;
        }
    }

    if (chef.equip && chef.equip.effect) {
        var equipCrit = null;
        if (chef.equip.skillDisp) {
            var m = EQUIP_CRIT_RE.exec(chef.equip.skillDisp);
            if (m) equipCrit = { rate: parseInt(m[1]) / 100, bonus: parseInt(m[2]) };
        }
        for (var i = 0; i < chef.equip.effect.length; i++) {
            var eff = chef.equip.effect[i];
            if (eff.type === "Material_Gain") {
                if (equipCrit && eff.value === equipCrit.bonus)
                    critExpected += equipCrit.rate * eff.value;
                else
                    flat += eff.value;
            }
            var cat = GATHER_CAT_TYPES[eff.type];
            if (cat) typeBonus[cat] += eff.value;
        }
    }

    if (chef.disk && chef.disk.ambers && chef.disk.level) {
        for (var i = 0; i < chef.disk.ambers.length; i++) {
            var amber = chef.disk.ambers[i];
            if (amber.data && amber.data.allEffect) {
                var lvlEffects = amber.data.allEffect[chef.disk.level - 1];
                if (lvlEffects) {
                    for (var j = 0; j < lvlEffects.length; j++) {
                        var eff = lvlEffects[j];
                        if (eff.type === "Material_Gain") flat += eff.value;
                        var cat = GATHER_CAT_TYPES[eff.type];
                        if (cat) typeBonus[cat] += eff.value;
                    }
                }
            }
        }
    }

    // Ultimate skill crit — ONLY if ultimate is unlocked
    if (chef.ultimate === "是" && chef.ultimateSkillEffect) {
        for (var i = 0; i < chef.ultimateSkillEffect.length; i++) {
            var eff = chef.ultimateSkillEffect[i];
            if (eff.type === "Material_Gain" && eff.value > 0) {
                var rate = 0;
                if (chef.ultimateSkillDisp) {
                    var m = chef.ultimateSkillDisp.match(/(\d+)%/);
                    if (m) rate = parseInt(m[1]) / 100;
                }
                critExpected += rate * eff.value;
                break;
            }
        }
    }

    return { effectiveGain: flat + critExpected, flat: flat, critExpected: critExpected, typeBonus: typeBonus };
}

function gatherGetPoints(chef, category) {
    switch (category) {
        case "Meat": return chef.meatVal || chef.meat || 0;
        case "Vegetable": return chef.vegVal || chef.veg || 0;
        case "Creation": return chef.creationVal || chef.creation || 0;
        case "Fish": return chef.fishVal || chef.fish || 0;
    }
    return 0;
}

// Count Green slots (total and empty) from diskDesc "红|绿|绿<br>最高5级"
function gatherGreenSlots(chef) {
    if (!chef.diskDesc) return { total: 0, empty: 0 };
    var parts = chef.diskDesc.split("<br>")[0].split("|");
    var total = 0, empty = 0;
    for (var i = 0; i < parts.length; i++) {
        if (parts[i] === "绿") {
            total++;
            if (!chef.disk || !chef.disk.ambers || !chef.disk.ambers[i] || !chef.disk.ambers[i].data)
                empty++;
        }
    }
    return { total: total, empty: empty };
}

function gatherRateForSkill(map, timeIdx, totalSkill) {
    var hours = map.time[timeIdx] / 3600;
    var total = 0, blocked = [];
    for (var i = 0; i < map.materials.length; i++) {
        var mat = map.materials[i];
        var qty = mat.quantity[timeIdx];
        if (mat.skill <= totalSkill) {
            total += (qty[0] + qty[1]) / 2;
        } else {
            blocked.push(mat.name + "(" + mat.skill + ")");
        }
    }
    return { rate: total / hours, blocked: blocked };
}


function gatherMaxSkill(map) {
    var max = 0;
    for (var i = 0; i < map.materials.length; i++) {
        if (map.materials[i].skill > max) max = map.materials[i].skill;
    }
    return max;
}

function gatherChefValue(entry, category) {
    return entry.effectiveGain + (entry.typeBonus[category] || 0);
}

function gatherOptimize() {
    var gd = calCustomRule.gameData;
    if (!gd || !gd.chefs || !gd.maps) {
        $("#gather-results").html("<div class='alert alert-warning'>未加载游戏数据或未导入存档</div>");
        return;
    }

    var includeDedicated = $("#chk-gather-include-dedicated").prop("checked");
    var allOwned = [], owned = [];
    for (var i = 0; i < gd.chefs.length; i++) {
        var c = gd.chefs[i];
        if (c.got !== "是") continue;
        var gv = gatherComputeChefValue(c);
        var entry = { chef: c, effectiveGain: gv.effectiveGain, flat: gv.flat, critExpected: gv.critExpected, typeBonus: gv.typeBonus };
        allOwned.push(entry);
        if (!includeDedicated) {
            var maxPts = Math.max(
                gatherGetPoints(c, "Meat"), gatherGetPoints(c, "Vegetable"),
                gatherGetPoints(c, "Creation"), gatherGetPoints(c, "Fish"));
            if (maxPts > 12) continue;
        }
        owned.push(entry);
    }
    if (owned.length === 0) {
        $("#gather-results").html("<div class='alert alert-warning'>未找到已拥有的厨师，请先导入存档</div>");
        return;
    }

    var mapData = [];
    for (var i = 0; i < gd.maps.length; i++) {
        var name = gd.maps[i].name;
        var cfg = MAP_DEFAULTS[name];
        if (!cfg) continue;
        mapData.push({
            map: gd.maps[i], name: name,
            category: MAP_CATEGORIES[name],
            timeIdx: cfg.timeIdx, chefCount: cfg.chefCount,
            baseRateAll: gatherRateForSkill(gd.maps[i], cfg.timeIdx, 999).rate,
            maxSkill: gatherMaxSkill(gd.maps[i])
        });
    }

    // 默认优先级：池塘/牧场等低产稀有素材地图优先分配好厨师
    var DEFAULT_ORDER = ["池塘", "牧场", "猪圈", "森林", "菜地", "菜棚", "鸡舍", "作坊"];
    var order = _gatherMapOrder || DEFAULT_ORDER;
    var orderMap = {};
    for (var i = 0; i < order.length; i++) orderMap[order[i]] = i;
    mapData.sort(function(a, b) {
        var ia = a.name in orderMap ? orderMap[a.name] : 99;
        var ib = b.name in orderMap ? orderMap[b.name] : 99;
        return ia - ib;
    });

    // Save current order for the reorder UI
    _gatherMapOrder = mapData.map(function(md) { return md.name; });

    // Greedy assignment with skill-point repair
    var available = owned.slice();
    var results = [];
    for (var mi = 0; mi < mapData.length; mi++) {
        var md = mapData[mi];
        var cat = md.category;
        available.sort(function(a, b) { return gatherChefValue(b, cat) - gatherChefValue(a, cat); });

        var team = available.splice(0, md.chefCount);

        var repairRounds = 0;
        while (repairRounds < md.chefCount) {
            var totalGain = 0, totalSkill = 0;
            for (var ti = 0; ti < team.length; ti++) {
                totalGain += gatherChefValue(team[ti], cat);
                totalSkill += gatherGetPoints(team[ti].chef, cat);
            }
            if (totalSkill >= md.maxSkill) break;

            var curRate = gatherRateForSkill(md.map, md.timeIdx, totalSkill).rate * (1 + totalGain / 100);
            var bestSwap = null;
            for (var ti = 0; ti < team.length; ti++) {
                var tSkill = gatherGetPoints(team[ti].chef, cat);
                var tVal = gatherChefValue(team[ti], cat);
                for (var ai = 0; ai < available.length; ai++) {
                    var aSkill = gatherGetPoints(available[ai].chef, cat);
                    if (aSkill <= tSkill) continue;
                    var aVal = gatherChefValue(available[ai], cat);
                    var newSkill = totalSkill - tSkill + aSkill;
                    var newGain = totalGain - tVal + aVal;
                    var newRate = gatherRateForSkill(md.map, md.timeIdx, newSkill).rate * (1 + newGain / 100);
                    if (newRate > curRate || (!bestSwap && newSkill > totalSkill)) {
                        curRate = newRate;
                        bestSwap = { ti: ti, ai: ai };
                    }
                }
            }
            if (!bestSwap) break;
            var tmp = team[bestSwap.ti];
            team[bestSwap.ti] = available[bestSwap.ai];
            available[bestSwap.ai] = tmp;
            repairRounds++;
        }

        var totalGain = 0, totalSkill = 0;
        for (var ti = 0; ti < team.length; ti++) {
            totalGain += gatherChefValue(team[ti], cat);
            totalSkill += gatherGetPoints(team[ti].chef, cat);
        }
        var gr = gatherRateForSkill(md.map, md.timeIdx, totalSkill);
        var enhancedRate = gr.rate * (1 + totalGain / 100);

        // Candidates: unassigned chefs with Green slots and decent bonus for this category
        var candidates = [];

        results.push({
            name: md.name, category: cat,
            chefCount: md.chefCount,
            hours: md.map.time[md.timeIdx] / 3600,
            baseRateAll: md.baseRateAll,
            baseRateGatherable: gr.rate,
            enhancedRate: enhancedRate,
            totalGain: totalGain,
            totalSkill: totalSkill, maxSkill: md.maxSkill,
            blocked: gr.blocked,
            team: team, candidates: candidates
        });
    }

    // Build assigned set and per-map candidates from unassigned pool
    var assignedIds = {};
    for (var i = 0; i < results.length; i++)
        for (var ti = 0; ti < results[i].team.length; ti++)
            assignedIds[results[i].team[ti].chef.chefId] = results[i].name;

    // Candidates: unified table of unassigned chefs with Green slots, showing all relevant maps
    var candidateTable = [];
    var catNames = { Meat: "肉", Vegetable: "菜", Creation: "面", Fish: "鱼" };
    var mapsByCat = {};
    for (var i = 0; i < results.length; i++) {
        var cat = results[i].category;
        if (!mapsByCat[cat]) mapsByCat[cat] = [];
        mapsByCat[cat].push(results[i].name);
    }
    for (var j = 0; j < owned.length; j++) {
        var e = owned[j];
        if (assignedIds[e.chef.chefId]) continue;
        var gs = gatherGreenSlots(e.chef);
        if (gs.total === 0) continue;
        var maps = [];
        var cats = ["Meat", "Vegetable", "Creation", "Fish"];
        for (var ci = 0; ci < cats.length; ci++) {
            var pts = gatherGetPoints(e.chef, cats[ci]);
            if (pts >= 2 && mapsByCat[cats[ci]])
                maps.push({ cat: cats[ci], catName: catNames[cats[ci]], pts: pts, mapNames: mapsByCat[cats[ci]] });
        }
        if (maps.length === 0) continue;
        // Sort score: best potential across all relevant categories
        var ml = e.chef.disk && e.chef.disk.maxLevel || 3;
        var bestPotential = 0;
        for (var mi = 0; mi < maps.length; mi++) {
            var pot = maps[mi].pts + gs.empty * (ml + 1);
            if (pot > bestPotential) bestPotential = pot;
        }
        candidateTable.push({
            entry: e, greenTotal: gs.total, greenEmpty: gs.empty,
            maps: maps, potential: bestPotential
        });
    }
    candidateTable.sort(function(a, b) {
        if (a.potential !== b.potential) return b.potential - a.potential;
        return b.entry.effectiveGain - a.entry.effectiveGain;
    });
    candidateTable = candidateTable.slice(0, 8);

    // Render
    var h = "<div style='margin-top:10px'>";

    // Priority hint
    h += "<div style='padding:4px 12px;color:#999;font-size:12px;margin-bottom:6px'>排列顺序 = 厨师分配优先级，点 ↑ 提升</div>";

    // Per-map results
    for (var i = 0; i < results.length; i++) {
        var r = results[i];
        var skillOk = r.totalSkill >= r.maxSkill;
        h += "<div style='padding:6px 12px;border:1px solid #eee;margin-bottom:6px'>";

        h += "<div>";
        if (i > 0)
            h += "<a href='#' class='gather-promote' data-idx='" + i + "' style='color:#888;text-decoration:none;margin-right:4px' title='提升优先级'>&uarr;</a>";
        h += "<b>" + r.name + "</b> (" + r.hours + "h, " + r.chefCount + "人)";
        h += " 点数: " + r.totalSkill + "/" + r.maxSkill;
        if (!skillOk) h += " <span style='color:red'>不足</span>";
        h += "</div>";

        h += "<div style='color:#555'>加成: ";
        for (var ti = 0; ti < r.team.length; ti++) {
            var t = r.team[ti], val = gatherChefValue(t, r.category);
            var pts = gatherGetPoints(t.chef, r.category);
            var tb = t.typeBonus[r.category] || 0;
            if (ti > 0) h += " + ";
            h += t.chef.name + "(" + pts + ", ";
            if (t.critExpected > 0)
                h += val.toFixed(0) + "%*";
            else
                h += val.toFixed(1) + "%";
            h += ")";
        }
        h += " = " + r.totalGain.toFixed(1) + "%</div>";

        if (r.blocked.length > 0)
            h += "<div style='color:red;font-size:12px'>不可采: " + r.blocked.join(", ") + "</div>";

        h += "<div style='color:#555'>";
        if (r.blocked.length > 0)
            h += "可采基础: " + r.baseRateGatherable.toFixed(2) + "/h (全部: " + r.baseRateAll.toFixed(2) + ")";
        else
            h += "基础: " + r.baseRateGatherable.toFixed(2) + "/h";
        h += " &times; (1 + " + r.totalGain.toFixed(1) + "%)";
        h += " = <b>" + r.enhancedRate.toFixed(2) + "/h</b></div>";

        h += "</div>";
    }

    // Candidate table
    if (candidateTable.length > 0) {
        h += "<div style='padding:6px 12px;border:1px solid #eee;margin-bottom:6px'>";
        h += "<div><b>候选</b> (绿槽潜力按2星玉估算)</div>";
        for (var ci = 0; ci < candidateTable.length; ci++) {
            var c = candidateTable[ci];
            var val = c.entry.effectiveGain;
            var rarity = c.entry.chef.rarity || "?";
            var ml = c.entry.chef.disk && c.entry.chef.disk.maxLevel || 3;
            var ptsPerSlot = ml + 1;
            var potentialGain = c.greenEmpty * ptsPerSlot;
            var gsLabel = c.greenEmpty + "空×" + ptsPerSlot + "=" + potentialGain;
            h += "<div style='color:#555;font-size:12px'>";
            h += c.entry.chef.name + " (" + rarity + "星, ";
            if (c.entry.critExpected > 0)
                h += val.toFixed(0) + "%*";
            else
                h += val.toFixed(1) + "%";
            h += ", " + c.greenTotal + "绿" + gsLabel + ") → ";
            for (var mi = 0; mi < c.maps.length; mi++) {
                if (mi > 0) h += ", ";
                h += c.maps[mi].mapNames.join("/") + " " + c.maps[mi].catName + "=" + c.maps[mi].pts;
            }
            h += "</div>";
        }
        h += "</div>";
    }

    h += "<div style='color:#999;font-size:12px;margin-top:6px'>* 含<a href='https://www.taptap.cn/moment/247819704842846587' target='_blank'>暴击期望</a> (暴击概率 &times; 额外素材%)</div>";
    h += "</div>";
    $("#gather-results").html(h);

    // Bind promote buttons — moves map one position up in priority
    $(".gather-promote").on("click", function(e) {
        e.preventDefault();
        var idx = parseInt($(this).data("idx"));
        var tmp = _gatherMapOrder[idx];
        _gatherMapOrder[idx] = _gatherMapOrder[idx - 1];
        _gatherMapOrder[idx - 1] = tmp;
        gatherOptimize();
    });
}

$(function() {
    $("#btn-gather-optimize").on("click", function() {
        _gatherMapOrder = null;
        gatherOptimize();
    });
    $("#chk-gather-include-dedicated").on("change", function() {
        if ($("#gather-results").children().length) gatherOptimize();
    });
});
